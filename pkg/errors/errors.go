package apperrors

import (
	"fmt"
	"net/http"
)

// HttpError - структура для кастомных HTTP-ошибок.
type HttpError struct {
	Code    int                    `json:"-"`
	Message string                 `json:"message"` // Сообщение для пользователя
	Err     error                  `json:"-"`       // Внутреннее сообщение для логов
	Context map[string]interface{} `json:"-"`       // Доп. данные
}

// Error - реализация интерфейса error
func (e *HttpError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("code: %d, message: %s, internal: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("code: %d, message: %s", e.Code, e.Message)
}

// Конструктор
func NewHttpError(code int, message string, err error, context map[string]interface{}) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Err:     err,
		Context: context,
	}
}

func NewBadRequestError(message string) *HttpError {
	if message == "" {
		return ErrBadRequest
	}
	return NewHttpError(http.StatusBadRequest, message, nil, nil)
}

// Предопределенные ошибки
var (
	ErrBadRequest           = NewHttpError(http.StatusBadRequest, "Неверный запрос", nil, nil)
	ErrValidation           = NewHttpError(http.StatusBadRequest, "Ошибка валидации данных", nil, nil)
	ErrUnauthorized         = NewHttpError(http.StatusUnauthorized, "Необходима авторизация", nil, nil)
	ErrForbidden            = NewHttpError(http.StatusForbidden, "Доступ запрещен", nil, nil)
	ErrNotFound             = NewHttpError(http.StatusNotFound, "Запрашиваемый ресурс не найден", nil, nil)
	ErrInternal             = NewHttpError(http.StatusInternalServerError, "Внутренняя ошибка сервера", nil, nil)
	ErrInvalidToken         = NewHttpError(http.StatusUnauthorized, "Недействительный токен", nil, nil)
	ErrTokenExpired         = NewHttpError(http.StatusUnauthorized, "Срок действия токена истек", nil, nil)
	ErrInvalidSigningMethod = NewHttpError(http.StatusUnauthorized, "Недействительный метод подписи токена", nil, nil)
	ErrConflict             = NewHttpError(http.StatusConflict, "Ресурс уже существует", nil, nil)
	ErrUserNotFound         = NewHttpError(http.StatusNotFound, "Пользователь не найден", nil, nil)
	ErrPriorityInUse        = NewHttpError(http.StatusBadRequest, "Приоритет используется и не может быть удалён", nil, nil)
	ErrStatusInUse          = NewHttpError(http.StatusBadRequest, "Статус используется и не может быть удалён", nil, nil)
	ErrInvalidCredentials   = NewHttpError(http.StatusUnauthorized, "Неверные учетные данные", nil, nil)
	ErrInternalServer       = NewHttpError(http.StatusInternalServerError, "Внутренняя ошибка сервера", nil, nil)
	ErrAccountLocked        = NewHttpError(http.StatusForbidden, "Аккаунт заблокирован", nil, nil)
	ErrTokenIsNotAccess     = NewHttpError(http.StatusUnauthorized, "Токен не является access токеном", nil, nil)
	ErrInvalidAuthHeader    = NewHttpError(http.StatusUnauthorized, "Недействительный заголовок авторизации", nil, nil)
	ErrEmptyAuthHeader      = NewHttpError(http.StatusUnauthorized, "Отсутствует заголовок авторизации", nil, nil)
)
