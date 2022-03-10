package common

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

const EncryptionBits = 4096
const CertValidity = time.Hour * 24 * 365

func ValidDaysRemaining(pem []byte) int {
	cert, err := x509.ParseCertificate(pem)
	if err != nil {
		Log.Warnf("could not parse certificate, %v", err)
		return -1
	}
	return int(cert.NotAfter.Sub(time.Now()).Hours() / 24)
}

func GenerateSelfSignedCA(commonName string) ([]byte, []byte, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 256)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %s", err)
	}
	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(CertValidity),
		Subject:               pkix.Name{CommonName: commonName},
		IsCA:                  true,
		MaxPathLenZero:        true,
		BasicConstraintsValid: true,
	}

	key, err := rsa.GenerateKey(rand.Reader, EncryptionBits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key: %s", err)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %s", err)
	}

	var certPem, keyPem bytes.Buffer
	if err := pem.Encode(&certPem, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, nil, fmt.Errorf("failed to encode certificate: %s", err)
	}
	if err := pem.Encode(&keyPem, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return nil, nil, fmt.Errorf("failed to encode key: %s", err)
	}

	return certPem.Bytes(), keyPem.Bytes(), nil
}

func GenerateSignedCert(commonName string, dnsNames []string, caPem []byte, caKey []byte) ([]byte, []byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, EncryptionBits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key: %s", err)
	}
	csrDerBytes, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate csr: %s", err.Error())
	}
	csr, err := x509.ParseCertificateRequest(csrDerBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse csr: %s", err.Error())
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 256)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %s", err)
	}
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(CertValidity),
		Subject:      pkix.Name{CommonName: commonName},
		DNSNames:     dnsNames,
	}
	caTlsPair, err := tls.X509KeyPair(caPem, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load CA key pair: %s", err)
	}

	ca, err := x509.ParseCertificate(caTlsPair.Certificate[0])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %s", err)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, ca, csr.PublicKey, caTlsPair.PrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %s", err)
	}

	var certPem, keyPem bytes.Buffer
	if err := pem.Encode(&certPem, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, nil, fmt.Errorf("failed to encode certificate: %s", err)
	}
	if err := pem.Encode(&keyPem, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return nil, nil, fmt.Errorf("failed to encode key: %s", err)
	}

	return certPem.Bytes(), keyPem.Bytes(), nil
}
