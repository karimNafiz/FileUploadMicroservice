package utility

import (
	"crypto/tls"
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
