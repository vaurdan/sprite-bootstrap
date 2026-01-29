// SSH host key management.
//
// Based on github.com/jbellerb/spritessh (MIT License)
// Copyright (c) 2026 jae beller

package sshserver

import (
	"bytes"
	"crypto/ed25519"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

var (
	defaultHostKeyName    = "sprite_bootstrap_host_ed25519_key"
	defaultHostKeyComment = "sprite@sprite-bootstrap"
)

// DefaultHostKeyPath returns the default path to the server host key.
func DefaultHostKeyPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".ssh", defaultHostKeyName), nil
}

// LoadHostKey loads the Ed25519 host key at the given path.
func LoadHostKey(path string) (ssh.Signer, error) {
	rawKey, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	priv, err := ssh.ParsePrivateKey(rawKey)
	if err != nil {
		return nil, fmt.Errorf("parse SSH private key: %w", err)
	}

	signer, err := ssh.NewSignerWithAlgorithms(
		priv.(ssh.AlgorithmSigner), []string{ssh.KeyAlgoED25519},
	)
	if err != nil {
		return nil, fmt.Errorf("expected Ed25519 private key: %w", err)
	}

	return signer, nil
}

// GenerateHostKey generates a new Ed25519 host key and writes it to the given path.
func GenerateHostKey(path string) (ssh.Signer, error) {
	rawPub, rawPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}
	pub, err := ssh.NewPublicKey(rawPub)
	if err != nil {
		return nil, err
	}
	priv, err := ssh.NewSignerFromKey(rawPriv)
	if err != nil {
		return nil, err
	}

	// ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}

	// write the private key
	privPem, err := ssh.MarshalPrivateKey(rawPriv, defaultHostKeyComment)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := pem.Encode(&buf, privPem); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		return nil, err
	}

	// write the public key, ignoring errors
	pubAuth := string(ssh.MarshalAuthorizedKey(pub))
	pubAuth = fmt.Sprintf("%s %s\n", strings.TrimSuffix(pubAuth, "\n"), defaultHostKeyComment)
	_ = os.WriteFile(path+".pub", []byte(pubAuth), 0644)

	return priv, nil
}

// LoadOrGenerateHostKey loads a host key from the given path, or generates one if it doesn't exist.
func LoadOrGenerateHostKey(path string) (ssh.Signer, error) {
	if path == "" {
		var err error
		path, err = DefaultHostKeyPath()
		if err != nil {
			return nil, fmt.Errorf("unable to find host key directory: %w", err)
		}
	}

	key, err := LoadHostKey(path)
	if err == nil {
		return key, nil
	}

	if os.IsNotExist(err) {
		key, err = GenerateHostKey(path)
		if err != nil {
			return nil, fmt.Errorf("failed to generate SSH host key: %w", err)
		}
		return key, nil
	}

	return nil, fmt.Errorf("failed to load SSH host key: %w", err)
}
