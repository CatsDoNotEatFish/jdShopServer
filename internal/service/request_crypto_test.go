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
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestRequestCipherDecryptsAndRejectsReplay(t *testing.T) {
	cipherService, err := NewRequestCipher(filepath.Join(t.TempDir(), "private.pem"))
	if err != nil {
		t.Fatal(err)
	}
	publicDER := cipherService.publicKey
	publicKeyAny, err := x509.ParsePKIXPublicKey(publicDER)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := publicKeyAny.(*rsa.PublicKey)
	requestID := "request-1"
	timestamp := time.Now().UnixMilli()
	path := "/api/v1/auth/login"
	plain := []byte(`{"phone":"13800138000","password":"secret"}`)
	aesKey := make([]byte, 32)
	if _, err := rand.Read(aesKey); err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(mustAES(t, aesKey))
	if err != nil {
		t.Fatal(err)
	}
	aad := []byte(path + "|" + strconv.FormatInt(timestamp, 10) + "|" + requestID)
	ciphertext := aead.Seal(nil, nonce, plain, aad)
	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, aesKey, []byte(requestID))
	if err != nil {
		t.Fatal(err)
	}
	envelope := EncryptedRequest{Version: 1, Algorithm: "RSA-OAEP-256+A256GCM", KeyID: cipherService.keyID, Key: base64.StdEncoding.EncodeToString(wrapped), Nonce: base64.StdEncoding.EncodeToString(nonce), Ciphertext: base64.StdEncoding.EncodeToString(ciphertext), Timestamp: timestamp, RequestID: requestID, Path: path}
	raw, _ := json.Marshal(envelope)
	got, err := cipherService.Decrypt(path, raw)
	if err != nil || string(got) != string(plain) {
		t.Fatalf("decrypt = %q, err=%v", got, err)
	}
	if _, err := cipherService.Decrypt(path, raw); err == nil {
		t.Fatal("expected replay to be rejected")
	}
}

func mustAES(t *testing.T, key []byte) cipher.Block {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	return block
}
