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

func TestAuthorizerOwnerHasAllOcservPermissions(t *testing.T) {
	t.Parallel()

	authorizer := NewDefaultAuthorizer()
	ocservPerms := []Permission{
		PermissionOcservRead,
		PermissionOcservServiceRead,
		PermissionOcservServiceManage,
		PermissionOcservSessionsRead,
		PermissionOcservSessionsDisconnect,
		PermissionOcservSessionsTerminate,
		PermissionOcservUsersRead,
		PermissionOcservUsersCreate,
		PermissionOcservUsersDelete,
		PermissionOcservUsersUpdate,
		PermissionOcservConfigRead,
	}
	for _, perm := range ocservPerms {
		if !authorizer.Allowed([]Role{RoleOwner}, perm) {
			t.Fatalf("expected owner to have permission %s", perm)
		}
	}
}

func TestAuthorizerViewerHasOcservReadPermissions(t *testing.T) {
	t.Parallel()

	authorizer := NewDefaultAuthorizer()
	readPerms := []Permission{
		PermissionOcservRead,
		PermissionOcservServiceRead,
		PermissionOcservSessionsRead,
		PermissionOcservUsersRead,
		PermissionOcservConfigRead,
	}
	for _, perm := range readPerms {
		if !authorizer.Allowed([]Role{RoleViewer}, perm) {
			t.Fatalf("expected viewer to have read permission %s", perm)
		}
	}
}

func TestAuthorizerViewerDeniedOcservMutatePermissions(t *testing.T) {
	t.Parallel()

	authorizer := NewDefaultAuthorizer()
	mutatePerms := []Permission{
		PermissionOcservServiceManage,
		PermissionOcservSessionsDisconnect,
		PermissionOcservSessionsTerminate,
		PermissionOcservUsersCreate,
		PermissionOcservUsersDelete,
		PermissionOcservUsersUpdate,
	}
	for _, perm := range mutatePerms {
		if authorizer.Allowed([]Role{RoleViewer}, perm) {
			t.Fatalf("expected viewer to be denied mutate permission %s", perm)
		}
	}
}

func TestAuthorizerAllowsOnlyOwnerToTerminateSessions(t *testing.T) {
	t.Parallel()

	authorizer := NewDefaultAuthorizer()

	if !authorizer.Allowed([]Role{RoleOwner}, PermissionOcservSessionsTerminate) {
		t.Fatal("expected owner role to terminate sessions")
	}
	if authorizer.Allowed([]Role{RoleViewer}, PermissionOcservSessionsTerminate) {
		t.Fatal("expected viewer role to be denied session termination")
	}
}
