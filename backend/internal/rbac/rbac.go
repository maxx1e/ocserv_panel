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
	PermissionReadDashboard   Permission = "dashboard:read"
	PermissionHistoryPurge    Permission = "history:purge"
	PermissionSettingsManage  Permission = "settings:manage"
	PermissionSettingsRead    Permission = "settings:read"
	PermissionSettingsRestart Permission = "settings:restart"

	// ocserv integration permissions
	PermissionOcservRead              Permission = "ocserv:read"
	PermissionOcservServiceRead       Permission = "ocserv:service:read"
	PermissionOcservServiceManage     Permission = "ocserv:service:manage"
	PermissionOcservSessionsRead       Permission = "ocserv:sessions:read"
	PermissionOcservSessionsDisconnect Permission = "ocserv:sessions:disconnect"
	PermissionOcservSessionsTerminate  Permission = "ocserv:sessions:terminate"
	PermissionOcservUsersRead         Permission = "ocserv:users:read"
	PermissionOcservUsersCreate       Permission = "ocserv:users:create"
	PermissionOcservUsersDelete       Permission = "ocserv:users:delete"
	PermissionOcservUsersUpdate       Permission = "ocserv:users:update"
	PermissionOcservConfigRead        Permission = "ocserv:config:read"
	PermissionOcservConfigEdit        Permission = "ocserv:config:edit"
	PermissionOcservConfigRollback    Permission = "ocserv:config:rollback"

	// Admin user management permissions
	PermissionAdminUsersRead   Permission = "admin:users:read"
	PermissionAdminUsersCreate Permission = "admin:users:create"
	PermissionAdminUsersUpdate Permission = "admin:users:update"
	PermissionAdminUsersDelete Permission = "admin:users:delete"
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
				PermissionReadDashboard:            {},
				PermissionHistoryPurge:             {},
				PermissionSettingsManage:           {},
				PermissionSettingsRead:             {},
				PermissionSettingsRestart:          {},
				PermissionOcservRead:               {},
				PermissionOcservServiceRead:        {},
				PermissionOcservServiceManage:      {},
				PermissionOcservSessionsRead:       {},
				PermissionOcservSessionsDisconnect: {},
				PermissionOcservSessionsTerminate:  {},
				PermissionOcservUsersRead:          {},
				PermissionOcservUsersCreate:        {},
				PermissionOcservUsersDelete:        {},
				PermissionOcservUsersUpdate:        {},
				PermissionOcservConfigRead:         {},
				PermissionOcservConfigEdit:         {},
				PermissionOcservConfigRollback:     {},
				PermissionAdminUsersRead:           {},
				PermissionAdminUsersCreate:         {},
				PermissionAdminUsersUpdate:         {},
				PermissionAdminUsersDelete:         {},
			},
			RoleViewer: {
				PermissionReadDashboard:       {},
				PermissionSettingsRead:        {},
				PermissionOcservRead:          {},
				PermissionOcservServiceRead:   {},
				PermissionOcservSessionsRead:  {},
				PermissionOcservUsersRead:     {},
				PermissionOcservConfigRead:    {},
				PermissionAdminUsersRead:      {},
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
