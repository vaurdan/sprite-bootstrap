// SSH server configuration and token resolution.
//
// Based on github.com/jbellerb/spritessh (MIT License)
// Copyright (c) 2026 jae beller

package sshserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	keyring "github.com/zalando/go-keyring"
)

var (
	errUnsupportedConfig = errors.New("unsupported config version")
	errNoUser            = errors.New("user not found")
	errNoURL             = errors.New("URL not found")
	errNoOrg             = errors.New("organization not found")
	errNoToken           = errors.New("missing access token")
)

var keyringService = "sprites-cli"

// Config is a simplified sprite configuration.
type Config struct {
	Version string `json:"version"`

	CurrentSelection *CurrentSelection     `json:"current_selection,omitempty"`
	URLs             map[string]*URLConfig `json:"urls"`

	Users       []*User `json:"users,omitempty"`
	CurrentUser string  `json:"current_user,omitempty"`
}

// CurrentSelection is the user's currently selected organization.
type CurrentSelection struct {
	URL string `json:"url"`
	Org string `json:"org"`
}

// URLConfig is the configuration for a specific API URL.
type URLConfig struct {
	URL  string          `json:"url"`
	Orgs map[string]*Org `json:"orgs"`
}

// Org is the configuration for a specific organization.
type Org struct {
	Name       string `json:"name"`
	KeyringKey string `json:"keyring_key"`
	Token      string `json:"token,omitempty"`
}

// User is information about an authenticated user.
type User struct {
	ID         string `json:"id"`
	ConfigPath string `json:"config_path"`
	TokenPath  string `json:"token_path"`
}

// LoadConfig loads the user's sprites config from a path.
func LoadConfig(path string) (*Config, error) {
	cfgRaw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(cfgRaw, &cfg); err != nil {
		return nil, err
	}

	if cfg.Version != "1" {
		return nil, errUnsupportedConfig
	}

	return &cfg, nil
}

// GetUser returns the info for a user with the given id.
func (c *Config) GetUser(id string) (*User, error) {
	for _, user := range c.Users {
		if user.ID == id {
			return user, nil
		}
	}

	return nil, errNoUser
}

// UserConfig returns the config for a user with the given id.
func (c *Config) UserConfig(id string) (*Config, error) {
	user, err := c.GetUser(id)
	if err != nil {
		return nil, err
	}

	userCfg, err := LoadConfig(user.ConfigPath)
	if err != nil {
		return nil, err
	}

	// overlay global selections
	userCfg.CurrentSelection = c.CurrentSelection
	userCfg.Users = []*User{user}
	userCfg.CurrentUser = id

	// merge global URLs
	for url, globalCfg := range c.URLs {
		if localCfg, ok := userCfg.URLs[url]; ok {
			for org, orgCfg := range globalCfg.Orgs {
				if _, ok := localCfg.Orgs[org]; !ok {
					localCfg.Orgs[org] = orgCfg
				}
			}
		} else {
			userCfg.URLs[url] = globalCfg
		}
	}

	return userCfg, nil
}

// GetOrg returns the info for an organization under the given URL and name.
func (c *Config) GetOrg(url, name string) (*Org, error) {
	urlCfg, ok := c.URLs[url]
	if !ok {
		return nil, errNoURL
	}

	org, ok := urlCfg.Orgs[name]
	if !ok {
		return nil, errNoOrg
	}

	return org, nil
}

// readKeyringToken reads a token from the system keyring.
func readKeyringToken(service, key string) (string, error) {
	value, err := keyring.Get(service, key)
	if err == nil {
		return value, nil
	}

	return readFallbackKeyringToken(service, key)
}

// readFallbackKeyringToken reads a token from the file-based keyring fallback.
func readFallbackKeyringToken(service, key string) (string, error) {
	keyPath, err := fallbackKeyringPath(service, key)
	if err != nil {
		return "", fmt.Errorf("unable to find fallback keyring: %w", err)
	}

	token, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read keyring file: %w", err)
	}

	return string(token), nil
}

// fallbackKeyringPath returns the file path to read the key from the file-based
// keyring fallback.
func fallbackKeyringPath(service, key string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// keys are stored at ~/.sprite/keyring/service/key
	// note that this is .sprite (singular), not .sprites (the config directory)
	keyringPath := filepath.Join(homeDir, ".sprite", "keyring")
	servicePath := filepath.Join(keyringPath, strings.ReplaceAll(service, ":", "-"))
	keyPath := filepath.Join(servicePath, strings.ReplaceAll(key, ":", "-"))

	return keyPath, nil
}

// GetToken returns the access token for the organization, possibly reading it
// from the system keyring.
func (c *Config) GetToken(org *Org) (string, error) {
	if org.KeyringKey != "" {
		service := keyringService
		if c.CurrentUser != "" {
			service = fmt.Sprintf("%s:%s", keyringService, c.CurrentUser)
		}
		return readKeyringToken(service, org.KeyringKey)
	} else if org.Token != "" {
		return org.Token, nil
	} else {
		return "", errNoToken
	}
}

// TokenOptions contains the minimal options needed for token resolution.
type TokenOptions struct {
	API          string
	AuthToken    string
	Organization string
}

// Resolve resolves the relevant API token from the global Sprites config.
func (o *TokenOptions) Resolve() error {
	if o.AuthToken != "" {
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to locate sprites configuration: %w", err)
	}
	cfg, err := LoadConfig(filepath.Join(homeDir, ".sprites", "sprites.json"))
	if err != nil {
		return fmt.Errorf("failed to read sprites config: %w", err)
	}

	// merge user config, if possible
	if cfg.CurrentUser != "" {
		cfg, err = cfg.UserConfig(cfg.CurrentUser)
		if err != nil {
			return fmt.Errorf("failed to read user sprites config: %w", err)
		}
	}

	return o.ResolveWithConfig(cfg)
}

// ResolveWithConfig resolves the relevant API token from the provided config.
func (o *TokenOptions) ResolveWithConfig(cfg *Config) error {
	if o.API == "" && cfg.CurrentSelection != nil {
		o.API = cfg.CurrentSelection.URL
	}
	if o.Organization == "" && cfg.CurrentSelection != nil {
		o.Organization = cfg.CurrentSelection.Org
	}

	if o.AuthToken == "" {
		org, err := cfg.GetOrg(o.API, o.Organization)
		if err != nil {
			return err
		}

		token, err := cfg.GetToken(org)
		if err != nil {
			return err
		}

		o.AuthToken = token
	}

	return nil
}
