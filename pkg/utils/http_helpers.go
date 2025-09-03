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
	"go.uber.org/zap"
)

// ... (структура HTTPResponse и ParseFilterFromQuery остаются БЕЗ ИЗМЕНЕНИЙ) ...
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
	// ... без изменений
	filterReq := types.Filter{Sort: make(map[string]string), Filter: make(map[string]interface{}), Limit: DefaultLimit, Page: 1}
	filterReq.Search = values.Get("search")
	for key, vals := range values {
		// Ищем параметры вида "sort[field_name]"
		if strings.HasPrefix(key, "sort[") && strings.HasSuffix(key, "]") && len(vals) > 0 {

			field := key[5 : len(key)-1]
			direction := strings.ToLower(vals[0])

			if direction == "asc" || direction == "desc" {
				filterReq.Sort[field] = direction
			}
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
	// ... (без изменений)
	response := &HTTPResponse{Status: true, Message: message}
	withPagination, _ := strconv.ParseBool(ctx.QueryParam("withPagination"))
	if withPagination && len(total) > 0 {
		filter := ParseFilterFromQuery(ctx.Request().URL.Query())
		var totalPages int
		if filter.Limit > 0 {
			totalPages = int(total[0]) / filter.Limit
			if int(total[0])%filter.Limit != 0 {
				totalPages++
			}
		}
		response.Body = map[string]interface{}{"list": body, "pagination": types.Pagination{TotalCount: total[0], Page: filter.Page, Limit: filter.Limit, TotalPages: totalPages}}
	} else {
		if body != nil {
			response.Body = body
		} else {
			response.Body = struct{}{}
		}
	}
	return ctx.JSON(code, response)
}

// ---- ИЗМЕНЕНИЯ ЗДЕСЬ ----
// Ошибка с логированием
func ErrorResponse(c echo.Context, err error, logger *zap.Logger) error {
	var httpErr *apperrors.HttpError
	if errors.As(err, &httpErr) {
		if httpErr.Err != nil {
			logger.Error("HTTP Error",
				zap.Int("code", httpErr.Code),
				zap.String("message", httpErr.Message),
				zap.Error(httpErr.Err),
				zap.Any("context", httpErr.Context),
			)
		}

		// Формируем базовый ответ
		response := map[string]interface{}{
			"status":  false,
			"message": httpErr.Message,
		}

		// Если есть details, добавляем их в ответ
		if httpErr.Details != nil {
			response["body"] = httpErr.Details
		}

		return c.JSON(httpErr.Code, response)
	}

	// Валидация
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		// ... (без изменений)
		var msgs []string
		for _, e := range validationErrors {
			msgs = append(msgs, fmt.Sprintf("Поле '%s' не прошло проверку '%s'", e.Field(), e.Tag()))
		}
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "Ошибка валидации: " + strings.Join(msgs, "; ")})
	}

	// Неожиданная ошибка
	logger.Error("Unexpected Error", zap.Error(err))
	return c.JSON(http.StatusInternalServerError, map[string]interface{}{
		"status":  false,
		"message": "Внутренняя ошибка сервера",
	})
}
