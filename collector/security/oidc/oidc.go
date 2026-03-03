package oidc

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/kescott027/diag-gateway/collector/security/rbac"
)

const (
	defaultRoleClaim    = "roles"
	defaultSubjectClaim = "sub"
	defaultClockSkew    = 30 * time.Second
)

// Config defines optional OIDC integration settings for control-plane auth.
type Config struct {
	Enabled        bool          `json:"enabled"`
	IssuerURL      string        `json:"issuer_url,omitempty"`
	Audience       string        `json:"audience,omitempty"`
	RoleClaim      string        `json:"role_claim,omitempty"`
	SubjectClaim   string        `json:"subject_claim,omitempty"`
	RequiredScopes []string      `json:"required_scopes,omitempty"`
	ClockSkew      time.Duration `json:"clock_skew,omitempty"`
}

// Normalize applies deterministic defaults.
func (c Config) Normalize() Config {
	out := c
	if strings.TrimSpace(out.RoleClaim) == "" {
		out.RoleClaim = defaultRoleClaim
	}
	if strings.TrimSpace(out.SubjectClaim) == "" {
		out.SubjectClaim = defaultSubjectClaim
	}
	if out.ClockSkew <= 0 {
		out.ClockSkew = defaultClockSkew
	}
	out.RequiredScopes = normalizeStrings(out.RequiredScopes)
	return out
}

// Validate enforces strict issuer/audience defaults when OIDC is enabled.
func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	cfg := c.Normalize()
	if _, err := parseHTTPSURL("issuer_url", cfg.IssuerURL); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Audience) == "" {
		return fmt.Errorf("audience is required when oidc is enabled")
	}
	if strings.TrimSpace(cfg.RoleClaim) == "" {
		return fmt.Errorf("role_claim is required when oidc is enabled")
	}
	if strings.TrimSpace(cfg.SubjectClaim) == "" {
		return fmt.Errorf("subject_claim is required when oidc is enabled")
	}
	return nil
}

// ProviderMetadata holds OIDC discovery metadata.
type ProviderMetadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint,omitempty"`
	TokenEndpoint         string `json:"token_endpoint,omitempty"`
	UserInfoEndpoint      string `json:"userinfo_endpoint,omitempty"`
	JWKSURI               string `json:"jwks_uri"`
}

// ParseProviderMetadata decodes discovery JSON.
func ParseProviderMetadata(raw []byte) (ProviderMetadata, error) {
	var out ProviderMetadata
	if err := json.Unmarshal(raw, &out); err != nil {
		return ProviderMetadata{}, fmt.Errorf("decode provider metadata: %w", err)
	}
	return out, nil
}

// Validate enforces required metadata fields and expected issuer.
func (m ProviderMetadata) Validate(expectedIssuer string) error {
	issuerURL, err := parseHTTPSURL("issuer", m.Issuer)
	if err != nil {
		return err
	}
	if expected := strings.TrimSpace(expectedIssuer); expected != "" {
		expectedURL, err := parseHTTPSURL("expected_issuer", expected)
		if err != nil {
			return err
		}
		if issuerURL.String() != expectedURL.String() {
			return fmt.Errorf("issuer mismatch: metadata=%q expected=%q", issuerURL.String(), expectedURL.String())
		}
	}
	if _, err := parseHTTPSURL("jwks_uri", m.JWKSURI); err != nil {
		return err
	}
	if strings.TrimSpace(m.AuthorizationEndpoint) != "" {
		if _, err := parseHTTPSURL("authorization_endpoint", m.AuthorizationEndpoint); err != nil {
			return err
		}
	}
	if strings.TrimSpace(m.TokenEndpoint) != "" {
		if _, err := parseHTTPSURL("token_endpoint", m.TokenEndpoint); err != nil {
			return err
		}
	}
	if strings.TrimSpace(m.UserInfoEndpoint) != "" {
		if _, err := parseHTTPSURL("userinfo_endpoint", m.UserInfoEndpoint); err != nil {
			return err
		}
	}
	return nil
}

// RoleMapper maps external role names to local RBAC roles.
type RoleMapper struct {
	Mapping map[string]rbac.Role `json:"mapping"`
}

// DefaultRoleMapper maps matching admin/operator/viewer role names.
func DefaultRoleMapper() RoleMapper {
	return RoleMapper{
		Mapping: map[string]rbac.Role{
			"admin":    rbac.RoleAdmin,
			"operator": rbac.RoleOperator,
			"viewer":   rbac.RoleViewer,
		},
	}
}

// MapRoles returns sorted, deduplicated local RBAC roles.
func (m RoleMapper) MapRoles(raw []string) []rbac.Role {
	if len(raw) == 0 {
		return nil
	}
	if m.Mapping == nil {
		m = DefaultRoleMapper()
	}
	set := make(map[rbac.Role]struct{})
	for _, role := range normalizeStrings(raw) {
		mapped, ok := m.Mapping[role]
		if !ok {
			continue
		}
		set[mapped] = struct{}{}
	}
	out := make([]rbac.Role, 0, len(set))
	for role := range set {
		out = append(out, role)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Identity is the normalized auth identity extracted from verified token claims.
type Identity struct {
	Subject   string      `json:"subject"`
	Issuer    string      `json:"issuer"`
	Audience  []string    `json:"audience"`
	Roles     []rbac.Role `json:"roles"`
	Scopes    []string    `json:"scopes,omitempty"`
	ExpiresAt time.Time   `json:"expires_at"`
}

// BuildIdentity validates claims against config and maps roles.
// Claims are expected to come from a signature-verified token.
func BuildIdentity(cfg Config, claims map[string]any, mapper RoleMapper, now time.Time) (Identity, error) {
	normalized := cfg.Normalize()
	if err := normalized.Validate(); err != nil {
		return Identity{}, err
	}

	subject, err := claimString(claims, normalized.SubjectClaim)
	if err != nil {
		return Identity{}, err
	}
	issuer, err := claimString(claims, "iss")
	if err != nil {
		return Identity{}, err
	}
	if !strings.EqualFold(issuer, strings.TrimSpace(normalized.IssuerURL)) {
		return Identity{}, fmt.Errorf("issuer mismatch: %q", issuer)
	}

	audience, err := claimStringSlice(claims, "aud")
	if err != nil {
		return Identity{}, err
	}
	if !slices.Contains(audience, strings.TrimSpace(normalized.Audience)) {
		return Identity{}, fmt.Errorf("audience mismatch: %q not in %v", normalized.Audience, audience)
	}

	expUnix, err := claimUnixTime(claims, "exp")
	if err != nil {
		return Identity{}, err
	}
	expiresAt := time.Unix(expUnix, 0).UTC()
	if now.UTC().After(expiresAt.Add(normalized.ClockSkew)) {
		return Identity{}, fmt.Errorf("token expired at %s", expiresAt.Format(time.RFC3339))
	}
	if nbfUnix, ok, err := claimOptionalUnixTime(claims, "nbf"); err != nil {
		return Identity{}, err
	} else if ok {
		notBefore := time.Unix(nbfUnix, 0).UTC()
		if now.UTC().Add(normalized.ClockSkew).Before(notBefore) {
			return Identity{}, fmt.Errorf("token not valid before %s", notBefore.Format(time.RFC3339))
		}
	}

	scopes := normalizeScopes(claims["scope"])
	for _, required := range normalized.RequiredScopes {
		if !slices.Contains(scopes, required) {
			return Identity{}, fmt.Errorf("missing required scope %q", required)
		}
	}

	roleClaims := normalizeScopes(claims[normalized.RoleClaim])
	roles := mapper.MapRoles(roleClaims)
	return Identity{
		Subject:   subject,
		Issuer:    issuer,
		Audience:  audience,
		Roles:     roles,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
	}, nil
}

func parseHTTPSURL(name, value string) (*url.URL, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("%s is required", name)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", name, err)
	}
	if parsed.Scheme != "https" {
		return nil, fmt.Errorf("%s must use https", name)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("%s host is required", name)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("%s must not include query or fragment", name)
	}
	return parsed, nil
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func normalizeScopes(raw any) []string {
	switch typed := raw.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return normalizeStrings(strings.Fields(typed))
	case []string:
		return normalizeStrings(typed)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				continue
			}
			values = append(values, text)
		}
		return normalizeStrings(values)
	default:
		return nil
	}
}

func claimString(claims map[string]any, key string) (string, error) {
	raw, ok := claims[key]
	if !ok {
		return "", fmt.Errorf("missing claim %q", key)
	}
	text, ok := raw.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("invalid claim %q", key)
	}
	return strings.TrimSpace(text), nil
}

func claimStringSlice(claims map[string]any, key string) ([]string, error) {
	raw, ok := claims[key]
	if !ok {
		return nil, fmt.Errorf("missing claim %q", key)
	}
	switch typed := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil, fmt.Errorf("invalid claim %q", key)
		}
		return []string{trimmed}, nil
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok || strings.TrimSpace(text) == "" {
				continue
			}
			out = append(out, strings.TrimSpace(text))
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("invalid claim %q", key)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("invalid claim %q", key)
	}
}

func claimUnixTime(claims map[string]any, key string) (int64, error) {
	raw, ok := claims[key]
	if !ok {
		return 0, fmt.Errorf("missing claim %q", key)
	}
	switch typed := raw.(type) {
	case float64:
		return int64(typed), nil
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case json.Number:
		return typed.Int64()
	default:
		return 0, fmt.Errorf("invalid claim %q", key)
	}
}

func claimOptionalUnixTime(claims map[string]any, key string) (int64, bool, error) {
	if _, ok := claims[key]; !ok {
		return 0, false, nil
	}
	value, err := claimUnixTime(claims, key)
	if err != nil {
		return 0, true, err
	}
	return value, true, nil
}
