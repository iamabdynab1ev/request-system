package utils

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type HTTPResponse struct {
	Status  bool        `json:"status"`
	Body    interface{} `json:"body,omitempty"`
	Message string      `json:"message"`
}

const (
	DefaultLimit = 100
	MaxLimit     = 500
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

	var httpErr *apperrors.HttpError
	if errors.As(err, &httpErr) {

		return c.JSON(httpErr.Code, map[string]interface{}{
			"status":  false,
			"message": httpErr.Message,
		})
	}
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		// Собираем красивое сообщение для фронтенда
		var errorMessages []string
		for _, e := range validationErrors {
			// Можно сделать более сложные переводы
			errorMessages = append(errorMessages,
				fmt.Sprintf("Поле '%s' не прошло проверку по правилу '%s'", e.Field(), e.Tag()))
		}

		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  false,
			"message": "Ошибка валидации: " + strings.Join(errorMessages, "; "),
		})
	}
	var echoHttpErr *echo.HTTPError
	if errors.As(err, &echoHttpErr) {

		return c.JSON(echoHttpErr.Code, map[string]interface{}{
			"status":  false,
			"message": echoHttpErr.Message,
		})
	}

	return c.JSON(http.StatusInternalServerError, map[string]interface{}{
		"status":  false,
		"message": "Внутренняя ошибка сервера",
	})
	
}
