package rbac

import "testing"

func TestAuthorizerAllowsKnownRolePermission(t *testing.T) {
	t.Parallel()

	authorizer := NewDefaultAuthorizer()

	if !authorizer.Allowed([]Role{RoleOwner}, PermissionReadDashboard) {
		t.Fatal("expected owner role to read dashboard")
	}
}

func TestAuthorizerDeniesMissingPermission(t *testing.T) {
	t.Parallel()

	authorizer := NewDefaultAuthorizer()

	if authorizer.Allowed([]Role{RoleViewer}, PermissionManageConfig) {
		t.Fatal("expected viewer role to be denied config management")
	}
}

func TestAuthorizerAllowsOnlyOwnerToPurgeHistory(t *testing.T) {
	t.Parallel()

	authorizer := NewDefaultAuthorizer()

	if !authorizer.Allowed([]Role{RoleOwner}, PermissionHistoryPurge) {
		t.Fatal("expected owner role to purge history")
	}
	if authorizer.Allowed([]Role{RoleViewer}, PermissionHistoryPurge) {
		t.Fatal("expected viewer role to be denied history purge")
	}
}

func TestAuthorizerDeniesUnknownRole(t *testing.T) {
	t.Parallel()

	authorizer := NewDefaultAuthorizer()

	if authorizer.Allowed([]Role{"unknown"}, PermissionReadDashboard) {
		t.Fatal("expected unknown role to be denied")
	}
}
