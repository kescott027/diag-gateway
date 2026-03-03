package secretstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStorePutGetDelete(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileStore(tmp)
	secretName := "agent_credentials"
	secretValue := []byte("super-secret")

	if err := store.Put(secretName, secretValue); err != nil {
		t.Fatalf("put failed: %v", err)
	}

	got, err := store.Get(secretName)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if string(got) != string(secretValue) {
		t.Fatalf("secret mismatch: got %q want %q", got, secretValue)
	}

	if err := store.Delete(secretName); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err = store.Get(secretName)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found after delete, got: %v", err)
	}
}

func TestFileStoreRejectsInsecurePermissions(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileStore(tmp)

	if err := store.Put("token", []byte("abc")); err != nil {
		t.Fatalf("put failed: %v", err)
	}

	path := filepath.Join(tmp, "token.secret")
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}

	_, err := store.Get("token")
	if !errors.Is(err, ErrInsecurePermissions) {
		t.Fatalf("expected insecure permission error, got: %v", err)
	}
}

func TestFileStoreRejectsInvalidName(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileStore(tmp)

	if err := store.Put("../bad", []byte("x")); !errors.Is(err, ErrInvalidSecretName) {
		t.Fatalf("expected invalid name error, got: %v", err)
	}
}
