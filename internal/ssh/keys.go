package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

// GenerateKey generates an ed25519 SSH key pair and saves it to the specified path
func GenerateKey(keyPath string) error {
	// Ensure directory exists with secure permissions
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Generate ed25519 key pair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Write private key in OpenSSH format
	privPEM := marshalOpenSSHPrivateKey(privKey, pubKey)
	if err := os.WriteFile(keyPath, privPEM, 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Write public key
	sshPub, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("failed to create SSH public key: %w", err)
	}
	pubBytes := ssh.MarshalAuthorizedKey(sshPub)
	if err := os.WriteFile(keyPath+".pub", pubBytes, 0644); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	return nil
}

// KeyExists checks if a key pair exists at the specified path
func KeyExists(keyPath string) bool {
	_, err := os.Stat(keyPath)
	return err == nil
}

// ReadPublicKey reads and returns the public key from the specified path
func ReadPublicKey(keyPath string) (string, error) {
	pubKeyPath := keyPath + ".pub"
	data, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read public key: %w", err)
	}
	return string(data), nil
}

// EnsureKey generates a key if it doesn't exist, returns the public key
func EnsureKey(keyPath string) (string, error) {
	if !KeyExists(keyPath) {
		if err := GenerateKey(keyPath); err != nil {
			return "", err
		}
	}
	return ReadPublicKey(keyPath)
}

// marshalOpenSSHPrivateKey marshals an ed25519 private key in OpenSSH format
func marshalOpenSSHPrivateKey(privKey ed25519.PrivateKey, pubKey ed25519.PublicKey) []byte {
	// OpenSSH private key format
	const authMagic = "openssh-key-v1\x00"

	// Generate check bytes (random, used for encryption verification)
	check := make([]byte, 4)
	rand.Read(check)
	checkInt := binary.BigEndian.Uint32(check)

	// Build the public key section
	pubKeyBytes := ssh.Marshal(struct {
		KeyType string
		PubKey  []byte
	}{
		KeyType: ssh.KeyAlgoED25519,
		PubKey:  pubKey,
	})

	// Build the private key section (with padding)
	privSection := ssh.Marshal(struct {
		Check1  uint32
		Check2  uint32
		KeyType string
		PubKey  []byte
		PrivKey []byte
		Comment string
	}{
		Check1:  checkInt,
		Check2:  checkInt,
		KeyType: ssh.KeyAlgoED25519,
		PubKey:  pubKey,
		PrivKey: privKey,
		Comment: "sprite-bootstrap",
	})

	// Add padding
	padLen := (8 - len(privSection)%8) % 8
	for i := 0; i < padLen; i++ {
		privSection = append(privSection, byte(i+1))
	}

	// Build the full key blob
	blob := ssh.Marshal(struct {
		CipherName   string
		KDFName      string
		KDFOptions   string
		NumKeys      uint32
		PubKey       []byte
		PrivKeyBlock []byte
	}{
		CipherName:   "none",
		KDFName:      "none",
		KDFOptions:   "",
		NumKeys:      1,
		PubKey:       pubKeyBytes,
		PrivKeyBlock: privSection,
	})

	// Prepend the magic header
	fullBlob := append([]byte(authMagic), blob...)

	// Encode as PEM
	return pem.EncodeToMemory(&pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: fullBlob,
	})
}
