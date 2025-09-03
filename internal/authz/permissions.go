package authz

const (
	Superuser = "superuser"

	OrdersCreate = "orders:create"
	OrdersView   = "orders:view"
	OrdersUpdate = "orders:update"
	OrdersDelete = "orders:delete"

	UsersCreate = "users:create"
	UsersView   = "users:view"
	UsersUpdate = "users:update"
	UsersDelete = "users:delete"

	RolesCreate = "roles:create"
	RolesView   = "roles:view"
	RolesUpdate = "roles:update"
	RolesDelete = "roles:delete"

	PermissionsView = "permissions:view"
	StatusesCreate  = "statuses:create"
	StatusesView    = "statuses:view"
	StatusesUpdate  = "statuses:update"
	StatusesDelete  = "statuses:delete"

	PrioritiesCreate = "priorities:create"
	PrioritiesView   = "priorities:view"
	PrioritiesUpdate = "priorities:update"
	PrioritiesDelete = "priorities:delete"

	DepartmentsCreate = "departments:create"
	DepartmentsView   = "departments:view"
	DepartmentsUpdate = "departments:update"
	DepartmentsDelete = "departments:delete"

	OtdelsCreate = "otdels:create"
	OtdelsView   = "otdels:view"
	OtdelsUpdate = "otdels:update"
	OtdelsDelete = "otdels:delete"

	BranchesCreate = "branches:create"
	BranchesView   = "branches:view"
	BranchesUpdate = "branches:update"
	BranchesDelete = "branches:delete"

	OfficesCreate = "offices:create"
	OfficesView   = "offices:view"
	OfficesUpdate = "offices:update"
	OfficesDelete = "offices:delete"

	EquipmentsCreate = "equipments:create"
	EquipmentsView   = "equipments:view"
	EquipmentsUpdate = "equipments:update"
	EquipmentsDelete = "equipments:delete"

	EquipmentTypesCreate = "equipment_types:create"
	EquipmentTypesView   = "equipment_types:view"
	EquipmentTypesUpdate = "equipment_types:update"
	EquipmentTypesDelete = "equipment_types:delete"

	PositionsCreate = "positions:create"
	PositionsView   = "positions:view"
	PositionsUpdate = "positions:update"
	PositionsDelete = "positions:delete"

	// СПЕЦИФИЧЕСКИЕ БИЗНЕС-ПРАВА
	OrdersDelegate     = "orders:delegate"
	UsersPasswordReset = "users:password:reset"
	ProfileUpdate      = "profile:update"
	PasswordUpdate     = "password:update"

	// SCOPES (Модификаторы Области)
	ScopeOwn        = "scope:own"
	ScopeDepartment = "scope:department"
	ScopeAll        = "scope:all"
)
