package authz

const (
	Superuser = "superuser"

	OrdersCreate = "order:create"
	OrdersView   = "order:view"
	OrdersUpdate = "order:update"
	OrdersDelete = "order:delete"

	UsersCreate = "user:create"
	UsersView   = "user:view"
	UsersUpdate = "user:update"
	UsersDelete = "user:delete"

	RolesCreate = "role:create"
	RolesView   = "role:view"
	RolesUpdate = "role:update"
	RolesDelete = "role:delete"

	PermissionsView = "permission:view"
	StatusesCreate  = "status:create"
	StatusesView    = "status:view"
	StatusesUpdate  = "status:update"
	StatusesDelete  = "status:delete"

	PrioritiesCreate = "priority:create"
	PrioritiesView   = "priority:view"
	PrioritiesUpdate = "priority:update"
	PrioritiesDelete = "priority:delete"

	DepartmentsCreate = "department:create"
	DepartmentsView   = "department:view"
	DepartmentsUpdate = "department:update"
	DepartmentsDelete = "department:delete"

	OtdelsCreate = "otdel:create"
	OtdelsView   = "otdel:view"
	OtdelsUpdate = "otdel:update"
	OtdelsDelete = "otdel:delete"

	BranchesCreate = "branch:create"
	BranchesView   = "branch:view"
	BranchesUpdate = "branch:update"
	BranchesDelete = "branch:delete"

	OfficesCreate = "office:create"
	OfficesView   = "office:view"
	OfficesUpdate = "office:update"
	OfficesDelete = "office:delete"

	EquipmentsCreate = "equipment:create"
	EquipmentsView   = "equipment:view"
	EquipmentsUpdate = "equipment:update"
	EquipmentsDelete = "equipment:delete"

	EquipmentTypesCreate = "equipment_type:create"
	EquipmentTypesView   = "equipment_type:view"
	EquipmentTypesUpdate = "equipment_type:update"
	EquipmentTypesDelete = "equipment_type:delete"

	PositionsCreate = "position:create"
	PositionsView   = "position:view"
	PositionsUpdate = "position:update"
	PositionsDelete = "position:delete"

	// СПЕЦИФИЧЕСКИЕ БИЗНЕС-ПРАВА
	OrdersDelegate     = "order:delegate"
	UsersPasswordReset = "user:password:reset"
	ProfileUpdate      = "profile:update"
	PasswordUpdate     = "password:update"

	// SCOPES (Модификаторы Области)
	ScopeOwn        = "scope:own"
	ScopeDepartment = "scope:department"
	ScopeAll        = "scope:all"
	OrdersReopen    = "order:reopen"
)
