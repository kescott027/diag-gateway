package enrollment

import (
	"crypto/x509"
	"sync"
	"time"

	"github.com/kescott027/diag-gateway/collector/security/admission"
	"github.com/kescott027/diag-gateway/collector/security/rotation"
)

// PullRenewalPolicy controls runtime credential renewal behavior.
type PullRenewalPolicy struct {
	RenewBefore   time.Duration
	MinIssueGap   time.Duration
	OverlapWindow time.Duration
}

func (p PullRenewalPolicy) withDefaults() PullRenewalPolicy {
	out := p
	if out.RenewBefore <= 0 {
		out.RenewBefore = 7 * 24 * time.Hour
	}
	if out.MinIssueGap <= 0 {
		out.MinIssueGap = 1 * time.Minute
	}
	if out.OverlapWindow <= 0 {
		out.OverlapWindow = 5 * time.Minute
	}
	return out
}

// SerialRevocations tracks when certificate serials become invalid.
type SerialRevocations struct {
	mu        sync.RWMutex
	effective map[string]time.Time
	now       func() time.Time
}

func NewSerialRevocations() *SerialRevocations {
	return &SerialRevocations{
		effective: make(map[string]time.Time),
		now:       time.Now,
	}
}

func (s *SerialRevocations) RevokeAfter(serial string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.effective[serial] = at.UTC()
}

func (s *SerialRevocations) IsRevoked(serial string) bool {
	s.mu.RLock()
	effective, ok := s.effective[serial]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	return !s.now().UTC().Before(effective)
}

// PullRenewer handles agent-initiated credential rotation over authenticated pull requests.
type PullRenewer struct {
	validator   *admission.Validator
	issuer      *Issuer
	revocations *SerialRevocations
	policy      PullRenewalPolicy
	now         func() time.Time

	mu         sync.Mutex
	lastIssued map[string]time.Time
}

func NewPullRenewer(validator *admission.Validator, issuer *Issuer, revocations *SerialRevocations, policy PullRenewalPolicy) *PullRenewer {
	if revocations == nil {
		revocations = NewSerialRevocations()
	}
	return &PullRenewer{
		validator:   validator,
		issuer:      issuer,
		revocations: revocations,
		policy:      policy.withDefaults(),
		now:         time.Now,
		lastIssued:  make(map[string]time.Time),
	}
}

// PullRenewIfNeeded validates identity and issues rotated credentials when renewal window is reached.
func (r *PullRenewer) PullRenewIfNeeded(peerCert *x509.Certificate) (CredentialBundle, bool, error) {
	now := r.now().UTC()
	sourceID, err := r.validator.ValidatePeerCertificate(peerCert)
	if err != nil {
		return CredentialBundle{}, false, err
	}

	renew, err := rotation.ShouldRenew(peerCert, now, r.policy.RenewBefore)
	if err != nil {
		return CredentialBundle{}, false, err
	}
	if !renew {
		return CredentialBundle{}, false, nil
	}

	r.mu.Lock()
	last, seen := r.lastIssued[sourceID]
	if seen && now.Sub(last) < r.policy.MinIssueGap {
		r.mu.Unlock()
		return CredentialBundle{}, false, nil
	}
	r.mu.Unlock()

	bundle, err := r.issuer.IssueForSource(sourceID)
	if err != nil {
		return CredentialBundle{}, false, err
	}

	if peerCert.SerialNumber != nil {
		revokeAt := now.Add(r.policy.OverlapWindow)
		r.revocations.RevokeAfter(peerCert.SerialNumber.String(), revokeAt)
	}

	r.mu.Lock()
	r.lastIssued[sourceID] = now
	r.mu.Unlock()

	return bundle, true, nil
}
