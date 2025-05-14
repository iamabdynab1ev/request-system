package utils

import (
	"errors"
	"net/http"
	"github.com/labstack/echo/v4"
)

type HttpResponse struct {
	Status  bool        `json:"status"`
	Body    interface{} `json:"body,omitempty"`
	Message string      `json:"message"`
}

func SuccessResponse(ctx echo.Context, body interface{}, message string, code int) error {
	var response *HttpResponse = &HttpResponse{
		Status:  true,
		Body:    body,
		Message: message,
	}
	return ctx.JSON(
		code,
		response,
	)
}

func ErrorResponse(ctx echo.Context, err error) error {
	var message string = err.Error()
	var code int = http.StatusInternalServerError

	for err, statusCode := range ErrorList {
		if errors.Is(err, err) {
			message = err.Error()
			code = statusCode
			break
		}
	}

	var response *HttpResponse = &HttpResponse{
		Status:  false,
		Body:    struct{}{},
		Message: message,
	}

	return ctx.JSON(
		code,
		response,
	)
}
