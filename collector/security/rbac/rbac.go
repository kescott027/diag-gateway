package rbac

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrUnknownRole   = errors.New("unknown role")
	ErrUnknownAction = errors.New("unknown action")
	ErrForbidden     = errors.New("forbidden")
)

// Role is a control-plane authorization role.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// Action is a control-plane permission target.
type Action string

const (
	ActionReadDashboard         Action = "read:dashboard"
	ActionReadSources           Action = "read:sources"
	ActionReadSearch            Action = "read:search"
	ActionReadLiveTail          Action = "read:livetail"
	ActionReadArtifacts         Action = "read:artifacts"
	ActionReadAudit             Action = "read:audit"
	ActionGenerateArtifactLinks Action = "write:artifact_links"
	ActionManageRetention       Action = "write:retention"
	ActionManageEnrollment      Action = "write:enrollment"
	ActionManageRevocation      Action = "write:revocation"
	ActionManageRouting         Action = "write:routing"
	ActionManageConfig          Action = "write:config"
	ActionManageRBAC            Action = "write:rbac"
)

// Policy provides deterministic role/action authorization decisions.
type Policy struct {
	grants map[Role]map[Action]struct{}
}

// DefaultPolicy returns the baseline admin/operator/viewer policy.
func DefaultPolicy() *Policy {
	all := allActions()
	admin := make(map[Action]struct{}, len(all))
	for _, action := range all {
		admin[action] = struct{}{}
	}
	operator := actionSet(
		ActionReadDashboard,
		ActionReadSources,
		ActionReadSearch,
		ActionReadLiveTail,
		ActionReadArtifacts,
		ActionReadAudit,
		ActionGenerateArtifactLinks,
		ActionManageRetention,
		ActionManageEnrollment,
		ActionManageRevocation,
		ActionManageRouting,
		ActionManageConfig,
	)
	viewer := actionSet(
		ActionReadDashboard,
		ActionReadSources,
		ActionReadSearch,
		ActionReadLiveTail,
		ActionReadArtifacts,
		ActionReadAudit,
	)
	return &Policy{
		grants: map[Role]map[Action]struct{}{
			RoleAdmin:    admin,
			RoleOperator: operator,
			RoleViewer:   viewer,
		},
	}
}

func actionSet(actions ...Action) map[Action]struct{} {
	out := make(map[Action]struct{}, len(actions))
	for _, action := range actions {
		out[action] = struct{}{}
	}
	return out
}

// ParseRole normalizes and validates a role string.
func ParseRole(raw string) (Role, error) {
	switch Role(strings.ToLower(strings.TrimSpace(raw))) {
	case RoleAdmin:
		return RoleAdmin, nil
	case RoleOperator:
		return RoleOperator, nil
	case RoleViewer:
		return RoleViewer, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownRole, raw)
	}
}

// ParseAction normalizes and validates an action string.
func ParseAction(raw string) (Action, error) {
	normalized := Action(strings.ToLower(strings.TrimSpace(raw)))
	for _, action := range allActions() {
		if action == normalized {
			return action, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownAction, raw)
}

// Can returns true only when role and action are both known and granted.
func (p *Policy) Can(role Role, action Action) bool {
	if p == nil {
		return false
	}
	if !isKnownRole(role) || !isKnownAction(action) {
		return false
	}
	actions, ok := p.grants[role]
	if !ok {
		return false
	}
	_, granted := actions[action]
	return granted
}

// Authorize returns explicit errors for unknown role/action or forbidden access.
func (p *Policy) Authorize(role Role, action Action) error {
	if !isKnownRole(role) {
		return fmt.Errorf("%w: %s", ErrUnknownRole, role)
	}
	if !isKnownAction(action) {
		return fmt.Errorf("%w: %s", ErrUnknownAction, action)
	}
	if p.Can(role, action) {
		return nil
	}
	return fmt.Errorf("%w: role=%s action=%s", ErrForbidden, role, action)
}

// Roles returns the sorted set of known roles.
func Roles() []Role {
	return []Role{RoleAdmin, RoleOperator, RoleViewer}
}

// Actions returns the sorted set of known actions.
func Actions() []Action {
	out := allActions()
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func allActions() []Action {
	return []Action{
		ActionReadDashboard,
		ActionReadSources,
		ActionReadSearch,
		ActionReadLiveTail,
		ActionReadArtifacts,
		ActionReadAudit,
		ActionGenerateArtifactLinks,
		ActionManageRetention,
		ActionManageEnrollment,
		ActionManageRevocation,
		ActionManageRouting,
		ActionManageConfig,
		ActionManageRBAC,
	}
}

func isKnownRole(role Role) bool {
	switch role {
	case RoleAdmin, RoleOperator, RoleViewer:
		return true
	default:
		return false
	}
}

func isKnownAction(action Action) bool {
	for _, known := range allActions() {
		if known == action {
			return true
		}
	}
	return false
}
