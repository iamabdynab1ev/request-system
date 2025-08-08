// internal/authz/permissions.go
package authz

// --- СПИСОК ВСЕХ ПЕРМИШЕНОВ В СИСТЕМЕ ---

const (
	// Глобальные
	Superuser = "superuser"

	// Заявки (Orders)
	OrdersCreate            = "orders:create"
	OrdersView              = "orders:view"
	OrdersUpdate            = "orders:update"
	OrdersDelete            = "orders:delete"
	OrdersDelegate          = "orders:delegate"
	OrdersAttachmentsCreate = "orders:attachments:create"
	OrdersAttachmentsDelete = "orders:attachments:delete"

	// Пользователи (Users)
	UsersCreate        = "users:create"
	UsersView          = "users:view"
	UsersUpdate        = "users:update"
	UsersDelete        = "users:delete"
	UsersPasswordReset = "users:password:reset"
	ProfileUpdate      = "profile:update"
	PasswordUpdate     = "password:update"

	// Роли (Roles)
	RolesCreate = "roles:create"
	RolesView   = "roles:view"
	RolesUpdate = "roles:update"
	RolesDelete = "roles:delete"

	// Пермишены (Permissions)
	PermissionsView = "permissions:view"

	// Структура (Structure)
	StructureCreate = "structure:create"
	StructureView   = "structure:view"
	StructureUpdate = "structure:update"
	StructureDelete = "structure:delete"

	// Справочники (Catalogs)
	CatalogsCreate = "catalogs:create"
	CatalogsView   = "catalogs:view"
	CatalogsUpdate = "catalogs:update"
	CatalogsDelete = "catalogs:delete"

	// Модификаторы Области (Scopes)
	ScopeOwn        = "scope:own"
	ScopeDepartment = "scope:department"
	ScopeBranch     = "scope:branch"
	ScopeAll        = "scope:all"
)
