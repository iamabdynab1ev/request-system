package contextkeys

type contextKey string

const (
	UserIDKey             contextKey = "UserID"
	UserPermissionsKey    contextKey = "UserPermissions"
	UserRoleIDKey         contextKey = "UserRoleID"
	RoleIDKey             contextKey = "RoleID"
	UserPermissionsMapKey contextKey = "userPermissionsMap"
)
