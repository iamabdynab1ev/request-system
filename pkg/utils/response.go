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
	response := &HttpResponse{
		Status:  true,
		Message: message,
	}

	withPagination, _ := strconv.ParseBool(ctx.QueryParam("withPagination"))

	if withPagination {
		limit, _, page := ParsePaginationParams(ctx.QueryParams())

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
	} else {

		if body != nil {
			response.Body = body
		} else {

			response.Body = struct{}{}
		}
	}

	return ctx.JSON(code, response)
}

func ErrorResponse(ctx echo.Context, incomingErr error) error {
	// Значения по умолчанию
	message := incomingErr.Error()
	code := http.StatusInternalServerError

	for errType, statusCode := range ErrorStatusCode {
		if errors.Is(incomingErr, errType) {

			message = errType.Error()
			code = statusCode
			break
		}
	}

	response := &HttpResponse{
		Status:  false,
		Message: message,
	}

	return ctx.JSON(code, response)
}
