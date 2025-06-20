package utils

import (
	"errors"
	"net/http"
	"request-system/pkg/types"
	"strconv"

	"github.com/labstack/echo/v4"
)

type HttpResponse struct {
	Status  bool        `json:"status"`
	Body    interface{} `json:"body,omitempty"`
	Message string      `json:"message"`
}

func SuccessResponse(ctx echo.Context, body interface{}, message string, code int, total ...uint64) error {
	var response *HttpResponse = &HttpResponse{
		Status:  true,
		Body:    struct{}{},
		Message: message,
	}

	withPagination, _ := strconv.ParseBool(ctx.QueryParam("withPagination"))
	if !withPagination {
		response.Body = body
	}

	if withPagination {

		var limit, _, page = ParsePaginationParams(ctx.QueryParams())

		var totalCount uint64 = 0
		if len(total) > 0 {
			totalCount = total[0]
		}

		response.Body = map[string]interface{}{
			"list": body,
			"pagination": types.Pagination{
				TotalCount: totalCount,
				Page:       page,
				Limit:      limit,
			},
		}
	}

	return ctx.JSON(
		code,
		response,
	)
}

func ErrorResponse(ctx echo.Context, err error) error {
	var message string = err.Error()
	var code int = http.StatusInternalServerError

	for err, statusCode := range ErrorStatusCode {
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
