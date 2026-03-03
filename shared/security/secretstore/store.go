package secretstore

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

var (
	ErrNotFound            = errors.New("secret not found")
	ErrInvalidSecretName   = errors.New("invalid secret name")
	ErrInsecurePermissions = errors.New("insecure secret file permissions")
)

var namePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// Store defines a minimal credential persistence interface.
type Store interface {
	Put(name string, value []byte) error
	Get(name string) ([]byte, error)
	Delete(name string) error
}

// FileStore persists credential material with restrictive file permissions.
type FileStore struct {
	BaseDir string
}

func NewFileStore(baseDir string) *FileStore {
	if baseDir == "" {
		baseDir = ".secrets"
	}
	return &FileStore{BaseDir: baseDir}
}

func (s *FileStore) Put(name string, value []byte) error {
	if err := validateName(name); err != nil {
		return err
	}
	if err := os.MkdirAll(s.BaseDir, 0o700); err != nil {
		return fmt.Errorf("ensure secret dir: %w", err)
	}

	target := s.secretPath(name)
	tmp := target + ".tmp"

	if err := os.WriteFile(tmp, value, 0o600); err != nil {
		return fmt.Errorf("write secret tmp file: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("promote secret file: %w", err)
	}
	return nil
}

func (s *FileStore) Get(name string) ([]byte, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	path := s.secretPath(name)
	stat, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("stat secret file: %w", err)
	}
	if stat.Mode().Perm()&0o077 != 0 {
		return nil, ErrInsecurePermissions
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read secret file: %w", err)
	}
	return data, nil
}

func (s *FileStore) Delete(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	path := s.secretPath(name)
	if err := os.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrNotFound
		}
		return fmt.Errorf("delete secret file: %w", err)
	}
	return nil
}

func (s *FileStore) secretPath(name string) string {
	return filepath.Join(s.BaseDir, name+".secret")
}

func validateName(name string) error {
	if !namePattern.MatchString(name) {
		return ErrInvalidSecretName
	}
	return nil
}
