package bootstrap

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultValidityDays = 365
)

// Config controls local CA and server certificate bootstrap output.
type Config struct {
	OutputDir        string
	CACommonName     string
	ServerCommonName string
	ValidForDays     int
}

// Paths identifies generated TLS artifacts.
type Paths struct {
	CAKeyPath      string
	CACertPath     string
	ServerKeyPath  string
	ServerCertPath string
}

func (c Config) withDefaults() Config {
	out := c
	if out.OutputDir == "" {
		out.OutputDir = "."
	}
	if out.CACommonName == "" {
		out.CACommonName = "diag-gateway-local-ca"
	}
	if out.ServerCommonName == "" {
		out.ServerCommonName = "collector.local"
	}
	if out.ValidForDays <= 0 {
		out.ValidForDays = defaultValidityDays
	}
	return out
}

func artifactPaths(base string) Paths {
	return Paths{
		CAKeyPath:      filepath.Join(base, "ca", "ca.key"),
		CACertPath:     filepath.Join(base, "ca", "ca.crt"),
		ServerKeyPath:  filepath.Join(base, "certs", "server.key"),
		ServerCertPath: filepath.Join(base, "certs", "server.crt"),
	}
}

// EnsureLocalPKI creates local CA and collector server cert/key files when absent.
// Existing artifacts are preserved to keep bootstrap idempotent.
func EnsureLocalPKI(cfg Config) (Paths, error) {
	cfg = cfg.withDefaults()
	paths := artifactPaths(cfg.OutputDir)

	if err := ensureDir(filepath.Dir(paths.CAKeyPath)); err != nil {
		return Paths{}, err
	}
	if err := ensureDir(filepath.Dir(paths.ServerKeyPath)); err != nil {
		return Paths{}, err
	}

	caCert, caKey, err := loadOrCreateCA(paths, cfg)
	if err != nil {
		return Paths{}, err
	}

	if err := loadOrCreateServerCert(paths, cfg, caCert, caKey); err != nil {
		return Paths{}, err
	}

	return paths, nil
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o700)
}

func loadOrCreateCA(paths Paths, cfg Config) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	certExists, keyExists := fileExists(paths.CACertPath), fileExists(paths.CAKeyPath)
	if certExists && keyExists {
		cert, key, err := loadCertAndKey(paths.CACertPath, paths.CAKeyPath)
		if err == nil {
			return cert, key, nil
		}
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate CA key: %w", err)
	}

	now := time.Now().UTC()
	tpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: cfg.CACommonName},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.Add(time.Duration(cfg.ValidForDays) * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create CA cert: %w", err)
	}

	if err := writeKey(paths.CAKeyPath, key); err != nil {
		return nil, nil, err
	}
	if err := writeCert(paths.CACertPath, der); err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, fmt.Errorf("parse generated CA cert: %w", err)
	}
	return cert, key, nil
}

func loadOrCreateServerCert(paths Paths, cfg Config, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) error {
	if fileExists(paths.ServerCertPath) && fileExists(paths.ServerKeyPath) {
		return nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate server key: %w", err)
	}

	now := time.Now().UTC()
	tpl := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: cfg.ServerCommonName},
		DNSNames:     []string{cfg.ServerCommonName, "localhost"},
		NotBefore:    now.Add(-1 * time.Hour),
		NotAfter:     now.Add(time.Duration(cfg.ValidForDays) * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, tpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("create server cert: %w", err)
	}

	if err := writeKey(paths.ServerKeyPath, key); err != nil {
		return err
	}
	if err := writeCert(paths.ServerCertPath, der); err != nil {
		return err
	}

	return nil
}

func loadCertAndKey(certPath, keyPath string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, nil, errors.New("invalid cert pem")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, errors.New("invalid key pem")
	}

	parsedKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	return cert, parsedKey, nil
}

func writeKey(path string, key *ecdsa.PrivateKey) error {
	bytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal key %s: %w", path, err)
	}
	block := &pem.Block{Type: "EC PRIVATE KEY", Bytes: bytes}
	return writePEM(path, block, 0o600)
}

func writeCert(path string, der []byte) error {
	block := &pem.Block{Type: "CERTIFICATE", Bytes: der}
	return writePEM(path, block, 0o644)
}

func writePEM(path string, block *pem.Block, perm os.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	if err := pem.Encode(file, block); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func serial() *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		// crypto/rand failures are unrecoverable; preserve deterministic fallback.
		return big.NewInt(time.Now().UnixNano())
	}
	return n
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
