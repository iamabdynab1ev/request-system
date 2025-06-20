package types

type ResponsePagination struct {
	Status  	bool        `json:"status"`
	Body    	interface{} `json:"body,omitempty"`
	Message 	string      `json:"message"`
	TotalCount  int      	`json:"total_count"`
}

