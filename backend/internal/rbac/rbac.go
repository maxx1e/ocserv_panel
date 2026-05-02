// Package rbac contains role and permission helpers for privileged actions.
package rbac

// Role identifies an administrative role.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleViewer Role = "viewer"
)

// Permission identifies an action guarded by authorization checks.
type Permission string

const (
	PermissionReadDashboard  Permission = "dashboard:read"
	PermissionManageUsers    Permission = "users:manage"
	PermissionManageConfig   Permission = "config:manage"
	PermissionManageService  Permission = "service:manage"
	PermissionHistoryPurge   Permission = "history:purge"
	PermissionSettingsManage Permission = "settings:manage"
)

// Authorizer evaluates role grants.
type Authorizer struct {
	grants map[Role]map[Permission]struct{}
}

// NewDefaultAuthorizer returns the initial local-first role grants.
func NewDefaultAuthorizer() Authorizer {
	return Authorizer{
		grants: map[Role]map[Permission]struct{}{
			RoleOwner: {
				PermissionReadDashboard:  {},
				PermissionManageUsers:    {},
				PermissionManageConfig:   {},
				PermissionManageService:  {},
				PermissionHistoryPurge:   {},
				PermissionSettingsManage: {},
			},
			RoleViewer: {
				PermissionReadDashboard: {},
			},
		},
	}
}

// Allowed reports whether any role grants the requested permission.
func (a Authorizer) Allowed(roles []Role, permission Permission) bool {
	for _, role := range roles {
		permissions, ok := a.grants[role]
		if !ok {
			continue
		}

		if _, ok := permissions[permission]; ok {
			return true
		}
	}

	return false
}
