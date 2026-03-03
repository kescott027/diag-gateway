package rbac

import (
	"errors"
	"testing"
)

func TestParseRole(t *testing.T) {
	role, err := ParseRole("  Admin ")
	if err != nil || role != RoleAdmin {
		t.Fatalf("expected admin role, got role=%q err=%v", role, err)
	}
	if _, err := ParseRole("guest"); !errors.Is(err, ErrUnknownRole) {
		t.Fatalf("expected unknown role error, got %v", err)
	}
}

func TestParseAction(t *testing.T) {
	action, err := ParseAction(" READ:Search ")
	if err != nil || action != ActionReadSearch {
		t.Fatalf("expected read:search action, got action=%q err=%v", action, err)
	}
	if _, err := ParseAction("write:unknown"); !errors.Is(err, ErrUnknownAction) {
		t.Fatalf("expected unknown action error, got %v", err)
	}
}

func TestDefaultPolicyMatrix(t *testing.T) {
	policy := DefaultPolicy()

	for _, action := range Actions() {
		if !policy.Can(RoleAdmin, action) {
			t.Fatalf("admin must be allowed for %s", action)
		}
	}

	if !policy.Can(RoleOperator, ActionManageRetention) {
		t.Fatalf("operator should be allowed to manage retention")
	}
	if policy.Can(RoleOperator, ActionManageRBAC) {
		t.Fatalf("operator should not be allowed to manage rbac")
	}
	if !policy.Can(RoleViewer, ActionReadDashboard) {
		t.Fatalf("viewer should be allowed to read dashboard")
	}
	if policy.Can(RoleViewer, ActionManageConfig) {
		t.Fatalf("viewer should not be allowed to manage config")
	}
	if policy.Can(Role("guest"), ActionReadDashboard) {
		t.Fatalf("unknown role must deny")
	}
	if policy.Can(RoleViewer, Action("write:nonexistent")) {
		t.Fatalf("unknown action must deny")
	}
}

func TestAuthorizeReturnsExplicitErrors(t *testing.T) {
	policy := DefaultPolicy()

	if err := policy.Authorize(RoleViewer, ActionReadSources); err != nil {
		t.Fatalf("expected viewer read sources allowed: %v", err)
	}
	if err := policy.Authorize(RoleOperator, ActionManageRBAC); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden for operator rbac, got %v", err)
	}
	if err := policy.Authorize(Role("guest"), ActionReadSources); !errors.Is(err, ErrUnknownRole) {
		t.Fatalf("expected unknown role, got %v", err)
	}
	if err := policy.Authorize(RoleViewer, Action("unknown")); !errors.Is(err, ErrUnknownAction) {
		t.Fatalf("expected unknown action, got %v", err)
	}
}
