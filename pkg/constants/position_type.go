package constants

// PositionType определяет тип должности в системе.
type PositionType string

// Константы для всех возможных типов должностей, используемых в маршрутизации.
const (
	PositionTypeHeadOfDepartment       PositionType = "HEAD_OF_DEPARTMENT"
	PositionTypeDeputyHeadOfDepartment PositionType = "DEPUTY_HEAD_OF_DEPARTMENT"
	PositionTypeHeadOfOtdel            PositionType = "HEAD_OF_OTDEL"
	PositionTypeDeputyHeadOfOtdel      PositionType = "DEPUTY_HEAD_OF_OTDEL"
	PositionTypeManager                PositionType = "MANAGER"
	PositionTypeSpecialist             PositionType = "SPECIALIST"
)

// PositionTypeNames связывает системные коды с человекочитаемыми названиями для API.
var PositionTypeNames = map[PositionType]string{
	PositionTypeHeadOfDepartment:       "Руководитель Департамента",
	PositionTypeDeputyHeadOfDepartment: "Заместитель Руководителя Департамента",
	PositionTypeHeadOfOtdel:            "Руководитель Отдела",
	PositionTypeDeputyHeadOfOtdel:      "Заместитель Руководителя Отдела",
	PositionTypeManager:                "Менеджер",
	PositionTypeSpecialist:             "Специалист",
}

var EscalationHierarchy = []PositionType{
	PositionTypeSpecialist,
	PositionTypeManager,
	PositionTypeDeputyHeadOfOtdel,
	PositionTypeHeadOfOtdel,
	PositionTypeDeputyHeadOfDepartment,
	PositionTypeHeadOfDepartment,
}
