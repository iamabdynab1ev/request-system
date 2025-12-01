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

	// --- 1. ОБРАБОТКА ПРОСТЫХ ТИПОВ (НЕ УКАЗАТЕЛЕЙ) ---
	if changes.Name != "" && merged.Name != changes.Name {
		merged.Name = changes.Name
		hasChanges = true
	}
	// ВНИМАНИЕ: DepartmentID отсюда УБРАЛИ, так как теперь это указатель!

	if changes.StatusID != 0 && merged.StatusID != changes.StatusID {
		merged.StatusID = changes.StatusID
		hasChanges = true
	}
	if changes.CreatorID != 0 && merged.CreatorID != changes.CreatorID {
		merged.CreatorID = changes.CreatorID
		hasChanges = true
	}

	// --- 2. ОБРАБОТКА УКАЗАТЕЛЕЙ (*uint64) ---

	// ДОБАВИЛИ СЮДА DepartmentID:
	if changes.DepartmentID != nil && (merged.DepartmentID == nil || *merged.DepartmentID != *changes.DepartmentID) {
		merged.DepartmentID = changes.DepartmentID
		hasChanges = true
	}

	if changes.OrderTypeID != nil && (merged.OrderTypeID == nil || *merged.OrderTypeID != *changes.OrderTypeID) {
		merged.OrderTypeID = changes.OrderTypeID
		hasChanges = true
	}
	if changes.OtdelID != nil && (merged.OtdelID == nil || *merged.OtdelID != *changes.OtdelID) {
		merged.OtdelID = changes.OtdelID
		hasChanges = true
	}
	if changes.BranchID != nil && (merged.BranchID == nil || *merged.BranchID != *changes.BranchID) {
		merged.BranchID = changes.BranchID
		hasChanges = true
	}
	if changes.OfficeID != nil && (merged.OfficeID == nil || *merged.OfficeID != *changes.OfficeID) {
		merged.OfficeID = changes.OfficeID
		hasChanges = true
	}
	if changes.EquipmentID != nil && (merged.EquipmentID == nil || *merged.EquipmentID != *changes.EquipmentID) {
		merged.EquipmentID = changes.EquipmentID
		hasChanges = true
	}
	if changes.EquipmentTypeID != nil && (merged.EquipmentTypeID == nil || *merged.EquipmentTypeID != *changes.EquipmentTypeID) {
		merged.EquipmentTypeID = changes.EquipmentTypeID
		hasChanges = true
	}
	if changes.PriorityID != nil && (merged.PriorityID == nil || *merged.PriorityID != *changes.PriorityID) {
		merged.PriorityID = changes.PriorityID
		hasChanges = true
	}
	if changes.ExecutorID != nil && (merged.ExecutorID == nil || *merged.ExecutorID != *changes.ExecutorID) {
		merged.ExecutorID = changes.ExecutorID
		hasChanges = true
	}
	if changes.ResolutionTimeSeconds != nil && (merged.ResolutionTimeSeconds == nil || *merged.ResolutionTimeSeconds != *changes.ResolutionTimeSeconds) {
		merged.ResolutionTimeSeconds = changes.ResolutionTimeSeconds
		hasChanges = true
	}
	if changes.FirstResponseTimeSeconds != nil && (merged.FirstResponseTimeSeconds == nil || *merged.FirstResponseTimeSeconds != *changes.FirstResponseTimeSeconds) {
		merged.FirstResponseTimeSeconds = changes.FirstResponseTimeSeconds
		hasChanges = true
	}

	// --- 3. ОБРАБОТКА ДРУГИХ УКАЗАТЕЛЕЙ (*string, *time.Time, *bool) ---
	if changes.Address != nil && (merged.Address == nil || *merged.Address != *changes.Address) {
		merged.Address = changes.Address
		hasChanges = true
	}
	if changes.Duration != nil && (merged.Duration == nil || !(*merged.Duration).Equal(*changes.Duration)) {
		merged.Duration = changes.Duration
		hasChanges = true
	}
	if changes.DeletedAt != nil && (merged.DeletedAt == nil || !merged.DeletedAt.Equal(*changes.DeletedAt)) {
		merged.DeletedAt = changes.DeletedAt
		hasChanges = true
	}
	if changes.CompletedAt != nil && (merged.CompletedAt == nil || !merged.CompletedAt.Equal(*changes.CompletedAt)) {
		merged.CompletedAt = changes.CompletedAt
		hasChanges = true
	}
	if changes.IsFirstContactResolution != nil && (merged.IsFirstContactResolution == nil || *merged.IsFirstContactResolution != *changes.IsFirstContactResolution) {
		merged.IsFirstContactResolution = changes.IsFirstContactResolution
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

	return filterReq
}

func SuccessResponse(ctx echo.Context, body interface{}, message string, code int, total ...uint64) error {
	response := &HTTPResponse{Status: true, Message: message}
	withPagination, _ := strconv.ParseBool(ctx.QueryParam("withPagination"))
	if withPagination && len(total) > 0 {
		filter := ParseFilterFromQuery(ctx.Request().URL.Query())
		totalPages := 0
		if filter.Limit > 0 {
			totalPages = int(total[0])/filter.Limit + (int(total[0]) % filter.Limit)
		}
		pagination := map[string]interface{}{
			"total_count": total[0],
			"page":        filter.Page,
			"limit":       filter.Limit,
			"total_pages": totalPages,
		}
		response.Body = map[string]interface{}{"list": body, "pagination": pagination}
	} else {
		response.Body = body
	}
	return ctx.JSON(code, response)
}

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

		response := map[string]interface{}{
			"status":  false,
			"message": httpErr.Message,
		}

		if httpErr.Details != nil {
			response["body"] = httpErr.Details
		}

		return c.JSON(httpErr.Code, response)
	}

	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		var msgs []string
		for _, e := range validationErrors {
			msgs = append(msgs, fmt.Sprintf("Поле '%s' не прошло проверку '%s'", e.Field(), e.Tag()))
		}
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "Ошибка валидации: " + strings.Join(msgs, "; ")})
	}

	logger.Error("Unexpected Error", zap.Error(err))
	return c.JSON(http.StatusInternalServerError, map[string]interface{}{
		"status":  false,
		"message": "Внутренняя ошибка сервера",
	})
}
