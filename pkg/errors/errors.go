package apperrors

import (
	"fmt"
	"net/http"
)

// HttpError - структура для кастомных HTTP-ошибок.
type HttpError struct {
	Code    int    `json:"-"`
	Message string `json:"message"`
	Err     error  `json:"-"` // Для внутреннего логирования
}

// Error - реализация интерфейса error.
func (e *HttpError) Error() string {
	return fmt.Sprintf("code: %d, message: %s", e.Code, e.Message)
}

// NewHttpError - конструктор для создания новых ошибок.
func NewHttpError(code int, message string, err error) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Предопределенные ошибки
var (
	// Ошибки валидации и запроса
	ErrBadRequest               = NewHttpError(http.StatusBadRequest, "Неверный запрос", nil)
	ErrValidation               = NewHttpError(http.StatusBadRequest, "Ошибка валидации данных", nil)
	ErrHeadOfDepartmentNotFound = NewHttpError(http.StatusBadRequest, "Руководитель отдела не найден", nil)

	// Ошибки аутентификации
	ErrUnauthorized       = NewHttpError(http.StatusUnauthorized, "Необходима авторизация", nil)
	ErrEmptyAuthHeader    = NewHttpError(http.StatusUnauthorized, "Заголовок авторизации отсутствует", nil)
	ErrInvalidAuthHeader  = NewHttpError(http.StatusUnauthorized, "Неверный формат заголовка авторизации", nil)
	ErrInvalidCredentials = NewHttpError(http.StatusUnauthorized, "Неверные учётные данные", nil)
	ErrAccountLocked      = NewHttpError(http.StatusLocked, "Аккаунт временно заблокирован", nil)

	// Ошибки авторизации
	ErrForbidden = NewHttpError(http.StatusForbidden, "Доступ запрещен", nil)

	// Ошибки токенов
	ErrInvalidToken      = NewHttpError(http.StatusUnauthorized, "Недопустимый или некорректный токен", nil)
	ErrTokenExpired      = NewHttpError(http.StatusUnauthorized, "Срок действия токена истёк", nil)
	ErrInvalidResetToken = NewHttpError(http.StatusBadRequest, "Невалидный или истекший токен сброса пароля", nil)
	ErrTokenIsNotAccess  = NewHttpError(http.StatusUnauthorized, "Попытка доступа с refresh токеном", nil)

	// Ошибки верификации
	ErrInvalidVerificationCode = NewHttpError(http.StatusBadRequest, "Неверный код верификации", nil)

	// Ошибки, связанные с данными
	ErrNotFound       = NewHttpError(http.StatusNotFound, "Запрашиваемый ресурс не найден", nil)
	ErrUserNotFound   = NewHttpError(http.StatusNotFound, "Пользователь не найден", nil)
	ErrStatusNotFound = NewHttpError(http.StatusNotFound, "Статус не найден", nil)

	// Внутренние ошибки
	ErrInternalServer          = NewHttpError(http.StatusInternalServerError, "Внутренняя ошибка сервера", nil)
	ErrUserIDNotFoundInContext = NewHttpError(http.StatusInternalServerError, "UserID не найден в контексте запроса", nil)
	ErrInvalidSigningMethod    = NewHttpError(http.StatusInternalServerError, "Внутренняя ошибка сервера: неверный метод подписи токена", nil)
	ErrInvalidUserID           = NewHttpError(http.StatusInternalServerError, "UserID в контексте имеет неверный тип или равен нулю", nil)
	ErrDefaultPriorityNotFound = NewHttpError(http.StatusInternalServerError, "Приоритет по умолчанию не найден", nil)
	ErrDefaultStatusNotFound   = NewHttpError(http.StatusBadRequest, "Статус по умолчанию не найден", nil)
)
