package constants

type PositionType string

const (
	PositionTypeHeadOfDepartment       PositionType = "HEAD_OF_DEPARTMENT"
	PositionTypeDeputyHeadOfDepartment PositionType = "DEPUTY_HEAD_OF_DEPARTMENT"

	// Один руководитель на Отдел
	PositionTypeManagerOfOtdel PositionType = "MANAGER_OF_OTDEL"

	PositionTypeBranchDirector       PositionType = "BRANCH_DIRECTOR"
	PositionTypeDeputyBranchDirector PositionType = "DEPUTY_BRANCH_DIRECTOR"

	PositionTypeHeadOfOffice       PositionType = "HEAD_OF_OFFICE"
	PositionTypeDeputyHeadOfOffice PositionType = "DEPUTY_HEAD_OF_OFFICE"

	PositionTypeSpecialist PositionType = "SPECIALIST"
)

// Русские названия для фронтенда
var PositionTypeNames = map[PositionType]string{
	PositionTypeHeadOfDepartment:       "Директор Департамента",
	PositionTypeDeputyHeadOfDepartment: "Заместитель Директора Департамента",
	PositionTypeManagerOfOtdel:         "Менеджер Отдела",

	PositionTypeBranchDirector:       "Директор Филиала",
	PositionTypeDeputyBranchDirector: "Заместитель Директора Филиала",

	PositionTypeHeadOfOffice:       "Руководитель ЦБО",
	PositionTypeDeputyHeadOfOffice: "Заместитель Руководителя ЦБО",

	PositionTypeSpecialist: "Специалист",
}

// ---- Новые Чистые Иерархии ----

// ИЕРАРХИЯ 1: Департамент
func GetDepartmentHierarchy() []PositionType {
	return []PositionType{
		PositionTypeHeadOfDepartment,
		PositionTypeDeputyHeadOfDepartment,
	}
}

// ИЕРАРХИЯ 2: Отдел (Только 1 уровень!)
func GetOtdelHierarchy() []PositionType {
	return []PositionType{
		PositionTypeManagerOfOtdel,
	}
}

// ИЕРАРХИЯ 3: Филиал
func GetBranchHierarchy() []PositionType {
	return []PositionType{
		PositionTypeBranchDirector,
		PositionTypeDeputyBranchDirector,
	}
}

// ИЕРАРХИЯ 4: Офис
func GetOfficeHierarchy() []PositionType {
	return []PositionType{
		PositionTypeHeadOfOffice,
		PositionTypeDeputyHeadOfOffice,
	}
}
