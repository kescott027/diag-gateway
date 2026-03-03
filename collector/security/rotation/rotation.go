package rotation

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

	"github.com/kescott027/diag-gateway/collector/security/bootstrap"
)

var (
	ErrMissingCertificate = errors.New("missing certificate")
)

// Policy defines server-certificate renewal behavior.
type Policy struct {
	RenewBefore  time.Duration
	ValidForDays int
}

func (p Policy) withDefaults() Policy {
	out := p
	if out.RenewBefore <= 0 {
		out.RenewBefore = 7 * 24 * time.Hour
	}
	if out.ValidForDays <= 0 {
		out.ValidForDays = 30
	}
	return out
}

// Rotator handles renewal checks and certificate replacement.
type Rotator struct {
	PKIPaths         bootstrap.Paths
	ServerCommonName string
	Policy           Policy
	now              func() time.Time
}

func NewRotator(paths bootstrap.Paths, commonName string, policy Policy) *Rotator {
	if commonName == "" {
		commonName = "collector.local"
	}
	return &Rotator{
		PKIPaths:         paths,
		ServerCommonName: commonName,
		Policy:           policy.withDefaults(),
		now:              time.Now,
	}
}

// ShouldRenew returns true when cert expiry enters renewal window.
func ShouldRenew(cert *x509.Certificate, now time.Time, renewBefore time.Duration) (bool, error) {
	if cert == nil {
		return false, ErrMissingCertificate
	}
	if renewBefore <= 0 {
		renewBefore = 7 * 24 * time.Hour
	}
	threshold := cert.NotAfter.Add(-renewBefore)
	return !now.Before(threshold), nil
}

// RotateServerCertificateIfNeeded rotates server cert when policy requires renewal.
func (r *Rotator) RotateServerCertificateIfNeeded() (bool, error) {
	cert, err := loadCert(r.PKIPaths.ServerCertPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := r.issueServerCertificate(); err != nil {
				return false, err
			}
			return true, nil
		}
		return false, err
	}

	renew, err := ShouldRenew(cert, r.now().UTC(), r.Policy.RenewBefore)
	if err != nil {
		return false, err
	}
	if !renew {
		return false, nil
	}
	if err := r.issueServerCertificate(); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Rotator) issueServerCertificate() error {
	caCert, caKey, err := loadCA(r.PKIPaths.CACertPath, r.PKIPaths.CAKeyPath)
	if err != nil {
		return err
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate server key: %w", err)
	}

	now := r.now().UTC()
	tpl := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: r.ServerCommonName},
		DNSNames:     []string{r.ServerCommonName, "localhost"},
		NotBefore:    now.Add(-1 * time.Hour),
		NotAfter:     now.Add(time.Duration(r.Policy.ValidForDays) * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, tpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("create server cert: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(r.PKIPaths.ServerKeyPath), 0o700); err != nil {
		return fmt.Errorf("ensure cert dir: %w", err)
	}
	if err := writeKey(r.PKIPaths.ServerKeyPath, key); err != nil {
		return err
	}
	if err := writeCert(r.PKIPaths.ServerCertPath, der); err != nil {
		return err
	}
	return nil
}

func loadCA(certPath, keyPath string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	cert, err := loadCert(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load ca cert: %w", err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read ca key: %w", err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, errors.New("invalid ca key pem")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ca key: %w", err)
	}
	return cert, key, nil
}

func loadCert(path string) (*x509.Certificate, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("invalid certificate pem")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}
	return cert, nil
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
		return big.NewInt(time.Now().UnixNano())
	}
	return n
}
