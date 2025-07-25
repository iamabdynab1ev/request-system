package types

type ResponsePagination struct {
	Status     bool        `json:"status"`
	Body       interface{} `json:"body,omitempty"`
	Message    string      `json:"message"`
	TotalCount int         `json:"total_count"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalPages int         `json:"total_pages"`
}
