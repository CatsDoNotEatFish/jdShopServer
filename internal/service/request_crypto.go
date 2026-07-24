package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const encryptedEnvelopeVersion = 1
const encryptedEnvelopeWindow = 2 * time.Minute

type EncryptedRequest struct {
	Version    int    `json:"v"`
	Algorithm  string `json:"alg"`
	KeyID      string `json:"kid"`
	Key        string `json:"key"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
	Timestamp  int64  `json:"ts"`
	RequestID  string `json:"request_id"`
	Path       string `json:"path"`
}

type PublicEncryptionKey struct {
	Version   int    `json:"v"`
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
	PublicKey string `json:"public_key"`
}

type RequestCipher struct {
	privateKey *rsa.PrivateKey
	publicKey  []byte
	keyID      string
	mu         sync.Mutex
	seen       map[string]time.Time
}

func NewRequestCipher(path string) (*RequestCipher, error) {
	if strings.TrimSpace(path) == "" {
		path = filepath.Join("data", "auth_encryption_private.pem")
	}
	privateKey, err := loadOrCreatePrivateKey(path)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal encryption public key: %w", err)
	}
	fingerprint := sha256.Sum256(der)
	return &RequestCipher{
		privateKey: privateKey,
		publicKey:  der,
		keyID:      base64.RawURLEncoding.EncodeToString(fingerprint[:8]),
		seen:       make(map[string]time.Time),
	}, nil
}

func (c *RequestCipher) PublicKey() PublicEncryptionKey {
	return PublicEncryptionKey{
		Version:   encryptedEnvelopeVersion,
		Algorithm: "RSA-OAEP-256+A256GCM",
		KeyID:     c.keyID,
		PublicKey: base64.StdEncoding.EncodeToString(c.publicKey),
	}
}

func (c *RequestCipher) Decrypt(path string, raw []byte) ([]byte, error) {
	var envelope EncryptedRequest
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, errors.New("encrypted request must be valid JSON")
	}
	if envelope.Version != encryptedEnvelopeVersion || envelope.Algorithm != "RSA-OAEP-256+A256GCM" || envelope.KeyID != c.keyID {
		return nil, errors.New("encrypted request key version is invalid")
	}
	if envelope.Path != path || strings.TrimSpace(envelope.RequestID) == "" {
		return nil, errors.New("encrypted request path or request id is invalid")
	}
	now := time.Now()
	stamp := time.UnixMilli(envelope.Timestamp)
	if stamp.IsZero() || now.Sub(stamp) > encryptedEnvelopeWindow || stamp.Sub(now) > encryptedEnvelopeWindow {
		return nil, errors.New("encrypted request has expired")
	}
	if !c.markRequestID(envelope.RequestID, now.Add(encryptedEnvelopeWindow)) {
		return nil, errors.New("encrypted request has already been used")
	}
	wrappedKey, err := base64.StdEncoding.DecodeString(envelope.Key)
	if err != nil {
		return nil, errors.New("encrypted request key is invalid")
	}
	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, c.privateKey, wrappedKey, []byte(envelope.RequestID))
	if err != nil {
		return nil, errors.New("encrypted request key cannot be decrypted")
	}
	nonce, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return nil, errors.New("encrypted request nonce is invalid")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return nil, errors.New("encrypted request body is invalid")
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, errors.New("encrypted request key size is invalid")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(nonce) != gcm.NonceSize() {
		return nil, errors.New("encrypted request nonce size is invalid")
	}
	aad := []byte(fmt.Sprintf("%s|%d|%s", envelope.Path, envelope.Timestamp, envelope.RequestID))
	plain, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, errors.New("encrypted request authentication failed")
	}
	return plain, nil
}

func (c *RequestCipher) markRequestID(id string, expires time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for key, deadline := range c.seen {
		if now.After(deadline) {
			delete(c.seen, key)
		}
	}
	if _, exists := c.seen[id]; exists {
		return false
	}
	c.seen[id] = expires
	return true
}

func loadOrCreatePrivateKey(path string) (*rsa.PrivateKey, error) {
	if data, err := os.ReadFile(path); err == nil {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, errors.New("invalid encryption private key PEM")
		}
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse encryption private key: %w", err)
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("encryption private key is not RSA")
		}
		return rsaKey, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate encryption private key: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, data, 0600); err != nil {
		return nil, err
	}
	return key, nil
}
