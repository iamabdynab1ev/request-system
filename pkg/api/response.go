package api

import (
	"github.com/labstack/echo/v4"

	apperrors "request-system/pkg/errors"
)

type Response[T any] struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Body    T      `json:"body,omitempty"`
}

type ListBody[T any] struct {
	List       []T             `json:"list"`
	Pagination *PaginationMeta `json:"pagination"`
}

type PaginationMeta struct {
	TotalCount uint64 `json:"total_count"`
	TotalPages int    `json:"total_pages"`
	Page       int    `json:"page"`
	Limit      int    `json:"limit"`
}

// SuccessOne — для возврата одного объекта
func SuccessOne[T any](c echo.Context, code int, message string, data T) error {
	return c.JSON(code, Response[T]{
		Status:  true,
		Message: message,
		Body:    data,
	})
}

func SuccessList[T any](c echo.Context, message string, list []T, total uint64, page, limit int) error {
	totalPages := 0
	if limit > 0 {
		totalPages = int((total + uint64(limit) - 1) / uint64(limit))
	}

	if list == nil {
		list = make([]T, 0)
	}

	body := ListBody[T]{
		List: list,
		Pagination: &PaginationMeta{
			TotalCount: total,
			TotalPages: totalPages,
			Page:       page,
			Limit:      limit,
		},
	}

	return c.JSON(200, Response[ListBody[T]]{
		Status:  true,
		Message: message,
		Body:    body,
	})
}

func ErrorResponse(c echo.Context, err error) error {
	code := 500
	msg := err.Error()

	// Для HttpError берем только пользовательское сообщение, без code и технических деталей
	if httpErr, ok := err.(*apperrors.HttpError); ok {
		code = httpErr.Code
		msg = httpErr.Message
	}

	return c.JSON(code, Response[any]{
		Status:  false,
		Message: msg,
	})
}
