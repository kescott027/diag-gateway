package enrollment

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
	"regexp"
	"time"

	"github.com/kescott027/diag-gateway/collector/security/bootstrap"
)

var (
	ErrInvalidSourceID = errors.New("invalid source id")
)

var sourceIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// CredentialConfig controls issued client-certificate defaults.
type CredentialConfig struct {
	PKIPaths      bootstrap.Paths
	ValidForDays  int
	IssuerOrgName string
}

// CredentialBundle contains PEM-encoded credential material for agent enrollment.
type CredentialBundle struct {
	SourceID  string
	CertPEM   []byte
	KeyPEM    []byte
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Exchanger redeems enrollment tokens and issues client credentials.
type Exchanger struct {
	tokens *Manager
	cfg    CredentialConfig
	now    func() time.Time
}

func NewExchanger(tokens *Manager, cfg CredentialConfig) *Exchanger {
	if cfg.ValidForDays <= 0 {
		cfg.ValidForDays = 30
	}
	if cfg.IssuerOrgName == "" {
		cfg.IssuerOrgName = "diag-gateway"
	}
	return &Exchanger{tokens: tokens, cfg: cfg, now: time.Now}
}

// ExchangeTokenForCredential redeems a valid token and issues a client cert for source identity.
func (e *Exchanger) ExchangeTokenForCredential(token, sourceID string) (CredentialBundle, error) {
	if !sourceIDPattern.MatchString(sourceID) {
		return CredentialBundle{}, ErrInvalidSourceID
	}
	if _, err := e.tokens.RedeemToken(token); err != nil {
		return CredentialBundle{}, err
	}

	caCert, caKey, err := loadCA(e.cfg.PKIPaths.CACertPath, e.cfg.PKIPaths.CAKeyPath)
	if err != nil {
		return CredentialBundle{}, err
	}

	now := e.now().UTC()
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return CredentialBundle{}, fmt.Errorf("generate client key: %w", err)
	}

	tpl := &x509.Certificate{
		SerialNumber: serial(),
		Subject: pkix.Name{
			CommonName:   sourceID,
			Organization: []string{e.cfg.IssuerOrgName},
		},
		DNSNames:    []string{sourceID},
		NotBefore:   now.Add(-1 * time.Hour),
		NotAfter:    now.Add(time.Duration(e.cfg.ValidForDays) * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, tpl, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		return CredentialBundle{}, fmt.Errorf("create client cert: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(clientKey)
	if err != nil {
		return CredentialBundle{}, fmt.Errorf("marshal client key: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return CredentialBundle{
		SourceID:  sourceID,
		CertPEM:   certPEM,
		KeyPEM:    keyPEM,
		IssuedAt:  now,
		ExpiresAt: tpl.NotAfter,
	}, nil
}

func loadCA(certPath, keyPath string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read ca cert: %w", err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read ca key: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, nil, errors.New("invalid ca cert pem")
	}
	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ca cert: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, errors.New("invalid ca key pem")
	}
	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ca key: %w", err)
	}

	return caCert, caKey, nil
}

func serial() *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return big.NewInt(time.Now().UnixNano())
	}
	return n
}
