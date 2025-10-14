package utils

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"request-system/internal/entities"
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
	DefaultLimit = 200
	MaxLimit     = 500
)

func StringPtr(s string) *string {
	return &s
}

func MergeOrders(original *entities.Order, changes entities.Order) (*entities.Order, bool) {
	hasChanges := false
	merged := *original // Создаем копию

	if changes.Name != "" && merged.Name != changes.Name {
		merged.Name = changes.Name
		hasChanges = true
	}
	if changes.Address != nil && merged.Address != nil && *merged.Address != *changes.Address {
		merged.Address = changes.Address
		hasChanges = true
	}
	if changes.DepartmentID != 0 && merged.DepartmentID != changes.DepartmentID {
		merged.DepartmentID = changes.DepartmentID
		hasChanges = true
	}
	if !AreUint64PointersEqual(merged.OtdelID, changes.OtdelID) {
		merged.OtdelID = changes.OtdelID
		hasChanges = true
	}
	if !AreUint64PointersEqual(merged.BranchID, changes.BranchID) {
		merged.BranchID = changes.BranchID
		hasChanges = true
	}
	if !AreUint64PointersEqual(merged.OfficeID, changes.OfficeID) {
		merged.OfficeID = changes.OfficeID
		hasChanges = true
	}
	if !AreUint64PointersEqual(merged.EquipmentID, changes.EquipmentID) {
		merged.EquipmentID = changes.EquipmentID
		hasChanges = true
	}
	if !AreUint64PointersEqual(merged.EquipmentTypeID, changes.EquipmentTypeID) {
		merged.EquipmentTypeID = changes.EquipmentTypeID
		hasChanges = true
	}

	if changes.StatusID != 0 && merged.StatusID != changes.StatusID {
		merged.StatusID = changes.StatusID
		hasChanges = true
	}
	if !AreUint64PointersEqual(merged.PriorityID, changes.PriorityID) {
		merged.PriorityID = changes.PriorityID
		hasChanges = true
	}
	if !AreUint64PointersEqual(merged.ExecutorID, changes.ExecutorID) {
		merged.ExecutorID = changes.ExecutorID
		hasChanges = true
	}

	if changes.Duration != nil {
		if merged.Duration == nil || !merged.Duration.Equal(*changes.Duration) {
			merged.Duration = changes.Duration
			hasChanges = true
		}
	} else if merged.Duration != nil {
		merged.Duration = nil
		hasChanges = true
	}

	return &merged, hasChanges
}

func ParseFilterFromQuery(values url.Values) types.Filter {
	filterReq := types.Filter{
		Sort:   make(map[string]string),
		Filter: make(map[string]interface{}),
		Limit:  DefaultLimit,
		Page:   1,
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

	if offsetStr := values.Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			filterReq.Offset = o
		}
	} else {
		filterReq.Offset = (filterReq.Page - 1) * filterReq.Limit
	}

	if values.Get("withPagination") == "true" {
		filterReq.WithPagination = true
	} else if values.Get("withPagination") == "false" {
		filterReq.WithPagination = false
	} else {
		filterReq.WithPagination = true
	}

	for key, vals := range values {
		if len(vals) == 0 || vals[0] == "" {
			continue
		}

		if key == "search" {
			filterReq.Search = vals[0]
			continue
		}

		if strings.HasPrefix(key, "sort[") && strings.HasSuffix(key, "]") {
			field := key[5 : len(key)-1]
			direction := strings.ToLower(vals[0])
			if direction == "asc" || direction == "desc" {
				filterReq.Sort[field] = direction
			}
			continue
		}

		if strings.HasPrefix(key, "filter[") && strings.HasSuffix(key, "]") {
			field := key[7 : len(key)-1]

			if existing, ok := filterReq.Filter[field]; ok {
				filterReq.Filter[field] = fmt.Sprintf("%v,%s", existing, vals[0])
			} else {
				filterReq.Filter[field] = vals[0]
			}
		}
	}
	// <<<--- КОНЕЦ НОВОЙ ЛОГИКИ ---

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
