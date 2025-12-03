package constants

type PositionType string

const (
	PositionTypeHeadOfDepartment       PositionType = "HEAD_OF_DEPARTMENT"
	PositionTypeDeputyHeadOfDepartment PositionType = "DEPUTY_HEAD_OF_DEPARTMENT"
	PositionTypeHeadOfOtdel            PositionType = "HEAD_OF_OTDEL"
	PositionTypeDeputyHeadOfOtdel      PositionType = "DEPUTY_HEAD_OF_OTDEL"

	PositionTypeDeputyBranchDirector PositionType = "DEPUTY_BRANCH_DIRECTOR"
	PositionTypeBranchDirector       PositionType = "BRANCH_DIRECTOR"
	PositionTypeDeputyHeadOfOffice   PositionType = "DEPUTY_HEAD_OF_OFFICE"
	PositionTypeHeadOfOffice         PositionType = "HEAD_OF_OFFICE"

	PositionTypeManager    PositionType = "MANAGER"
	PositionTypeSpecialist PositionType = "SPECIALIST"
)

var PositionTypeNames = map[PositionType]string{
	PositionTypeHeadOfDepartment:       "Директор Департамента",
	PositionTypeDeputyHeadOfDepartment: "Заместитель Директора Департамента",
	PositionTypeHeadOfOtdel:            "Менеджер Отдела",
	PositionTypeDeputyHeadOfOtdel:      "",

	PositionTypeDeputyBranchDirector: "Заместитель Директора Филиала",
	PositionTypeBranchDirector:       "Директор Филиала",
	PositionTypeDeputyHeadOfOffice:   "Заместитель Руководителя ЦБО",
	PositionTypeHeadOfOffice:         "Руководитель ЦБО",

	PositionTypeManager:    "Менеджер",
	PositionTypeSpecialist: "Специалист",
}

func GetAscendingHierarchy() []PositionType {
	return []PositionType{
		PositionTypeManager,
		PositionTypeDeputyHeadOfOtdel,
		PositionTypeHeadOfOtdel,
		PositionTypeDeputyHeadOfDepartment,
		PositionTypeHeadOfDepartment,
	}
}

func GetDescendingHierarchy() []PositionType {
	asc := GetAscendingHierarchy()
	desc := make([]PositionType, len(asc))
	for i := 0; i < len(asc); i++ {
		desc[i] = asc[len(asc)-1-i]
	}
	return desc
}

func GetAscendingBranchHierarchy() []PositionType {
	return []PositionType{
		PositionTypeSpecialist,
		PositionTypeManager,
		PositionTypeDeputyHeadOfOffice,
		PositionTypeHeadOfOffice,
		PositionTypeDeputyBranchDirector,
		PositionTypeBranchDirector,
	}
}

// Добавляем: Иерархия для ФИЛИАЛОВ (Сверху-Вниз) - для автоназначения
func GetDescendingBranchHierarchy() []PositionType {
	asc := GetAscendingBranchHierarchy()
	desc := make([]PositionType, len(asc))
	for i := 0; i < len(asc); i++ {
		desc[i] = asc[len(asc)-1-i]
	}
	return desc
}
