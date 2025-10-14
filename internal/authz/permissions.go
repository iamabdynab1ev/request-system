package authz

const (
	// ЗАЯВКИ: ОСНОВНЫЕ ДЕЙСТВИЯ
	OrdersCreate = "order:create"
	OrdersView   = "order:view"
	OrdersUpdate = "order:update"
	OrdersDelete = "order:delete"

	// ПОЛЬЗОВАТЕЛИ
	UsersCreate        = "user:create"
	UsersView          = "user:view"
	UsersUpdate        = "user:update"
	UsersDelete        = "user:delete"
	UsersPasswordReset = "user:password:reset"

	// ПРОФИЛЬ
	ProfileUpdate  = "profile:update"
	PasswordUpdate = "password:update"

	// РОЛИ И ПРАВА
	RolesCreate = "role:create"
	RolesView   = "role:view"
	RolesUpdate = "role:update"
	RolesDelete = "role:delete"

	// СПРАВОЧНИКИ (полный набор CRUD для каждого)
	StatusesCreate = "status:create"
	StatusesView   = "status:view"
	StatusesUpdate = "status:update"
	StatusesDelete = "status:delete"

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

	// --- ПОЛЯ ПРИ СОЗДАНИИ ---
	OrdersCreateName            = "order:create:name"
	OrdersCreateAddress         = "order:create:address"
	OrdersCreateDepartmentID    = "order:create:department_id"
	OrdersCreateOtdelID         = "order:create:otdel_id"
	OrdersCreateBranchID        = "order:create:branch_id"
	OrdersCreateOfficeID        = "order:create:office_id"
	OrdersCreateEquipmentID     = "order:create:equipment_id"
	OrdersCreateEquipmentTypeID = "order:create:equipment_type_id"
	OrdersCreateExecutorID      = "order:create:executor_id"
	OrdersCreatePriorityID      = "order:create:priority_id"
	OrdersCreateDuration        = "order:create:duration"
	OrdersCreateComment         = "order:create:comment"
	OrdersCreateFile            = "order:create:file"

	// --- ПОЛЯ ПРИ ОБНОВЛЕНИИ ---
	OrdersUpdateName            = "order:update:name"
	OrdersUpdateAddress         = "order:update:address"
	OrdersUpdateDepartmentID    = "order:update:department_id"
	OrdersUpdateOtdelID         = "order:update:otdel_id"
	OrdersUpdateBranchID        = "order:update:branch_id"
	OrdersUpdateOfficeID        = "order:update:office_id"
	OrdersUpdateEquipmentID     = "order:update:equipment_id"
	OrdersUpdateEquipmentTypeID = "order:update:equipment_type_id"
	OrdersUpdateExecutorID      = "order:update:executor_id"
	OrdersUpdateStatusID        = "order:update:status_id"
	OrdersUpdatePriorityID      = "order:update:priority_id"
	OrdersUpdateDuration        = "order:update:duration"
	OrdersUpdateComment         = "order:update:comment"
	OrdersUpdateFile            = "order:update:file"
	OrdersUpdateReopen          = "order:update:reopen"

	ScopeOwn        = "scope:own"
	ScopeOtdel      = "scope:otdel"
	ScopeOffice     = "scope:office"
	ScopeBranch     = "scope:branch"
	ScopeDepartment = "scope:department"
	ScopeAll        = "scope:all"
	ScopeAllView    = "scope:all_view"

	OrdersUpdateInOtdelScope      = "order:update_in_otdel_scope"
	OrdersUpdateInOfficeScope     = "order:update_in_office_scope"
	OrdersUpdateInBranchScope     = "order:update_in_branch_scope"
	OrdersUpdateInDepartmentScope = "order:update_in_department_scope"

	// ПРИВИЛЕГИИ (ПРАВА ДОСТУПА)
	PermissionsCreate = "permission:create"
	PermissionsUpdate = "permission:update"
	PermissionsDelete = "permission:delete"
	PermissionsView   = "permission:view"

	// МАРШРУТИЗАЦИЯ ЗАЯВОК
	OrderRuleCreate = "order_rule:create"
	OrderRuleUpdate = "order_rule:update"
	OrderRuleDelete = "order_rule:delete"
	OrderRuleView   = "order_rule:view"

	// ДОЛЖНОСТИ
	PositionsCreate = "position:create"
	PositionsView   = "position:view"
	PositionsUpdate = "position:update"
	PositionsDelete = "position:delete"

	// --- ТИПЫ ЗАЯВОК ---
	OrderTypesCreate = "order_type:create"
	OrderTypesView   = "order_type:view"
	OrderTypesUpdate = "order_type:update"
	OrderTypesDelete = "order_type:delete"
	// ОТЧЕТЫ
	ReportView = "report:view"
)
