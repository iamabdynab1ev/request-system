package utils

type HttpResponse struct {
	Status  bool        `json:"status"`
	Message string      `json:"message"`
	Body    interface{} `json:"body"`
}

func SuccessResponse(data interface{}) *HttpResponse {
	return &HttpResponse{
		Status:  true,
		Body:    data,
		Message: "Success",
	}
}

func ErrorResponse(data interface{}) *HttpResponse {
	return &HttpResponse{
		Status:  false,
		Body:    data,
		Message: "Error",
	}
}
