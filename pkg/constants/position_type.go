package constants

type PositionType string

const (
	PositionTypeHeadOfDepartment       PositionType = "HEAD_OF_DEPARTMENT"
	PositionTypeDeputyHeadOfDepartment PositionType = "DEPUTY_HEAD_OF_DEPARTMENT"
	PositionTypeManagerOfOtdel         PositionType = "MANAGER_OF_OTDEL"
	PositionTypeBranchDirector         PositionType = "BRANCH_DIRECTOR"
	PositionTypeDeputyBranchDirector   PositionType = "DEPUTY_BRANCH_DIRECTOR"
	PositionTypeHeadOfOffice           PositionType = "HEAD_OF_OFFICE"
	PositionTypeDeputyHeadOfOffice     PositionType = "DEPUTY_HEAD_OF_OFFICE"
)

var PositionTypeNames = map[PositionType]string{
	PositionTypeHeadOfDepartment:       "Директор Департамента",
	PositionTypeDeputyHeadOfDepartment: "Зам. Директора Департамента",
	PositionTypeManagerOfOtdel:         "Менеджер Отдела",
	PositionTypeBranchDirector:         "Директор Филиала",
	PositionTypeDeputyBranchDirector:   "Зам. Директора Филиала",
	PositionTypeHeadOfOffice:           "Руководитель Офиса (ЦБО)",
	PositionTypeDeputyHeadOfOffice:     "Зам. Руководителя Офиса (ЦБО)",
}
