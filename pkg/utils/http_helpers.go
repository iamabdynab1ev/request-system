// Файл: pkg/utils/http_helpers.go

package utils

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"

	"github.com/labstack/echo/v4"
)

type HTTPResponse struct {
	Status  bool        `json:"status"`
	Body    interface{} `json:"body,omitempty"`
	Message string      `json:"message"`
}

const (
	DefaultLimit = 10
	MaxLimit     = 100
)

func ParseFilterFromQuery(values url.Values) types.Filter {
	filterReq := types.Filter{
		Sort:   make(map[string]string),
		Filter: make(map[string]interface{}),
		Limit:  DefaultLimit,
		Page:   1,
	}

	filterReq.Search = values.Get("search")

	for key, vals := range values {
		if strings.HasPrefix(key, "sort[") && strings.HasSuffix(key, "]") && len(vals) > 0 {
			field := key[5 : len(key)-1]
			direction := strings.ToLower(vals[0])
			if direction == "asc" || direction == "desc" {
				filterReq.Sort[field] = direction
			}
		}
	}

	for key, vals := range values {
		if strings.HasPrefix(key, "filter[") && strings.HasSuffix(key, "]") && len(vals) > 0 {
			field := key[7 : len(key)-1]
			filterReq.Filter[field] = vals[0]
		}
	}

	if limitStr := values.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			if l > MaxLimit {
				filterReq.Limit = MaxLimit
			} else {
				filterReq.Limit = l
			}
		}
	}

	if pageStr := values.Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			filterReq.Page = p
		}
	}

	if values.Get("offset") != "" {
		if o, err := strconv.Atoi(values.Get("offset")); err == nil && o >= 0 {
			filterReq.Offset = o
		}
	} else {
		filterReq.Offset = (filterReq.Page - 1) * filterReq.Limit
	}

	if values.Get("withPagination") == "true" {
		filterReq.WithPagination = true
	}

	return filterReq
}

func SuccessResponse(ctx echo.Context, body interface{}, message string, code int, total ...uint64) error {
	response := &HTTPResponse{
		Status:  true,
		Message: message,
	}

	withPagination, _ := strconv.ParseBool(ctx.QueryParam("withPagination"))

	if withPagination && len(total) > 0 {
		filter := ParseFilterFromQuery(ctx.Request().URL.Query())

		response.Body = map[string]interface{}{
			"list": body,
			"pagination": types.Pagination{
				TotalCount: total[0],
				Page:       filter.Page,
				Limit:      filter.Limit,

				TotalPages: int(total[0]/uint64(filter.Limit)) + 1,
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

func ErrorResponse(c echo.Context, err error) error {
	var statusCode int
	var message string

	switch err {
	case apperrors.ErrEmptyAuthHeader, apperrors.ErrInvalidAuthHeader, apperrors.ErrInvalidToken, apperrors.ErrTokenExpired, apperrors.ErrTokenIsNotAccess:
		statusCode = http.StatusUnauthorized
		message = "Необходима авторизация"
	case apperrors.ErrForbidden:
		statusCode = http.StatusForbidden
		message = "Доступ запрещён"
	default:
		statusCode = http.StatusInternalServerError
		message = "Произошла ошибка: " + err.Error()
	}

	return c.JSON(statusCode, map[string]interface{}{
		"status":  false,
		"message": message,
	})
}
