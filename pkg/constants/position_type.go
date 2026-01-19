package constants

type PositionType string

const (
	// --- Департамент ---
	PositionTypeHeadOfDepartment       PositionType = "HEAD_OF_DEPARTMENT"
	PositionTypeDeputyHeadOfDepartment PositionType = "DEPUTY_HEAD_OF_DEPARTMENT"

	// --- Отдел (БЫВШИЙ ManagerOfOtdel) ---
	PositionTypeHeadOfOtdel            PositionType = "HEAD_OF_OTDEL"        
	PositionTypeDeputyHeadOfOtdel      PositionType = "DEPUTY_HEAD_OF_OTDEL" 

	// --- Филиал ---
	PositionTypeBranchDirector         PositionType = "BRANCH_DIRECTOR"
	PositionTypeDeputyBranchDirector   PositionType = "DEPUTY_BRANCH_DIRECTOR"

	// --- Офис (ЦБО) ---
	PositionTypeHeadOfOffice           PositionType = "HEAD_OF_OFFICE"
	PositionTypeDeputyHeadOfOffice     PositionType = "DEPUTY_HEAD_OF_OFFICE"

	// --- Остальные ---
	PositionTypeManager                PositionType = "MANAGER" 
	PositionTypeSpecialist             PositionType = "SPECIALIST"
)

var PositionTypeNames = map[PositionType]string{
	PositionTypeHeadOfDepartment:       "Руководитель департамента",
	PositionTypeDeputyHeadOfDepartment: "Заместитель руководителя департамента",
	
	PositionTypeHeadOfOtdel:            "Руководитель отдела",
	PositionTypeDeputyHeadOfOtdel:      "Заместитель руководителя отдела",

	PositionTypeBranchDirector:         "Директор филиала",
	PositionTypeDeputyBranchDirector:   "Заместитель директора филиала",
	
	PositionTypeHeadOfOffice:           "Руководитель офиса",
	PositionTypeDeputyHeadOfOffice:     "Заместитель руководителя офиса",
	
	PositionTypeManager:                "Менеджер",
	PositionTypeSpecialist:             "Специалист",
}
