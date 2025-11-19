package dto


type ReportItemDTO struct {
	OrderID          int64  `json:"order_id"`
	CreatorFio       string `json:"creator_fio"`
	CreatedAt        string `json:"created_at"`
	OrderTypeName    string `json:"order_type_name"`
	PriorityName     string `json:"priority_name"`
	StatusName       string `json:"status_name"`
	OrderName        string `json:"order_name"`
	ResponsibleFio   string `json:"responsible_fio"`
	DelegatedAt      string `json:"delegated_at"`
	ExecutorFio      string `json:"executor_fio"`
	CompletedAt      string `json:"completed_at"`
	ResolutionTime   string `json:"resolution_time"`
	SLAStatus        string `json:"sla_status"`
	SourceDepartment string `json:"source_department"`
	Comment          string `json:"comment"`
}
