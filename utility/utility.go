package utility

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/google/uuid"
)

func LoadTLSCertKey(cert_dst string, key_dst string) (tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(cert_dst, key_dst)
	if err != nil {
		log.Fatalf("could not load key pair: %v", err)
	}
	return cert, err
}

func NewUploadID() string {
	return uuid.New().String()
}

func GetVanillaServiceID() func() string {
	start_index := 0
	return func() string {
		start_index++
		return fmt.Sprintf("service:%d", start_index)
	}
}

func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}
func GetKeyString(key []byte) string {
	return base64.RawURLEncoding.EncodeToString(key)
}

func ComputeHMAC(data, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}
