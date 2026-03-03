package oidc

import (
	"errors"
	"testing"
	"time"

	"github.com/kescott027/diag-gateway/collector/security/rbac"
)

func TestConfigValidate(t *testing.T) {
	disabled := Config{}
	if err := disabled.Validate(); err != nil {
		t.Fatalf("disabled config should validate: %v", err)
	}

	enabled := Config{
		Enabled:      true,
		IssuerURL:    "https://issuer.example",
		Audience:     "diag-gateway",
		RoleClaim:    "roles",
		SubjectClaim: "sub",
	}
	if err := enabled.Validate(); err != nil {
		t.Fatalf("valid enabled config failed validation: %v", err)
	}

	invalid := enabled
	invalid.IssuerURL = "http://issuer.example"
	if err := invalid.Validate(); err == nil {
		t.Fatalf("expected invalid http issuer")
	}
}

func TestParseProviderMetadataAndValidate(t *testing.T) {
	raw := []byte(`{"issuer":"https://issuer.example","jwks_uri":"https://issuer.example/keys","authorization_endpoint":"https://issuer.example/auth","token_endpoint":"https://issuer.example/token"}`)
	meta, err := ParseProviderMetadata(raw)
	if err != nil {
		t.Fatalf("parse metadata failed: %v", err)
	}
	if err := meta.Validate("https://issuer.example"); err != nil {
		t.Fatalf("metadata validation failed: %v", err)
	}
	if err := meta.Validate("https://other.example"); err == nil {
		t.Fatalf("expected issuer mismatch")
	}
}

func TestRoleMapper(t *testing.T) {
	mapper := DefaultRoleMapper()
	roles := mapper.MapRoles([]string{"viewer", "admin", "admin", "unknown"})
	if len(roles) != 2 || roles[0] != rbac.RoleAdmin || roles[1] != rbac.RoleViewer {
		t.Fatalf("unexpected role mapping: %+v", roles)
	}
}

func TestBuildIdentitySuccess(t *testing.T) {
	cfg := Config{
		Enabled:        true,
		IssuerURL:      "https://issuer.example",
		Audience:       "diag-gateway",
		RoleClaim:      "roles",
		SubjectClaim:   "sub",
		RequiredScopes: []string{"openid", "profile"},
		ClockSkew:      10 * time.Second,
	}
	now := time.Date(2026, 3, 3, 14, 40, 0, 0, time.UTC)
	claims := map[string]any{
		"iss":   "https://issuer.example",
		"sub":   "user-1",
		"aud":   []any{"diag-gateway"},
		"exp":   float64(now.Add(time.Minute).Unix()),
		"scope": "openid profile",
		"roles": []any{"operator", "viewer"},
	}
	identity, err := BuildIdentity(cfg, claims, DefaultRoleMapper(), now)
	if err != nil {
		t.Fatalf("build identity failed: %v", err)
	}
	if identity.Subject != "user-1" || len(identity.Roles) != 2 {
		t.Fatalf("unexpected identity: %+v", identity)
	}
}

func TestBuildIdentityRejectsExpiredAndScopeMismatch(t *testing.T) {
	cfg := Config{
		Enabled:        true,
		IssuerURL:      "https://issuer.example",
		Audience:       "diag-gateway",
		RequiredScopes: []string{"openid"},
	}
	now := time.Date(2026, 3, 3, 14, 40, 0, 0, time.UTC)

	expiredClaims := map[string]any{
		"iss": "https://issuer.example",
		"sub": "user-1",
		"aud": "diag-gateway",
		"exp": float64(now.Add(-2 * time.Minute).Unix()),
	}
	if _, err := BuildIdentity(cfg, expiredClaims, DefaultRoleMapper(), now); err == nil {
		t.Fatalf("expected expired token failure")
	}

	scopeClaims := map[string]any{
		"iss":   "https://issuer.example",
		"sub":   "user-1",
		"aud":   "diag-gateway",
		"exp":   float64(now.Add(time.Minute).Unix()),
		"scope": "email",
	}
	if _, err := BuildIdentity(cfg, scopeClaims, DefaultRoleMapper(), now); err == nil {
		t.Fatalf("expected missing required scope failure")
	}
}

func TestBuildIdentityRejectsAudienceMismatch(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		IssuerURL: "https://issuer.example",
		Audience:  "diag-gateway",
	}
	now := time.Date(2026, 3, 3, 14, 40, 0, 0, time.UTC)
	claims := map[string]any{
		"iss": "https://issuer.example",
		"sub": "user-1",
		"aud": "other-app",
		"exp": float64(now.Add(time.Minute).Unix()),
	}
	if _, err := BuildIdentity(cfg, claims, DefaultRoleMapper(), now); err == nil {
		t.Fatalf("expected audience mismatch failure")
	}
}

func TestParseActionAndRoleErrorsRemainTyped(t *testing.T) {
	if _, err := rbac.ParseRole("bad"); !errors.Is(err, rbac.ErrUnknownRole) {
		t.Fatalf("expected unknown role error")
	}
}
