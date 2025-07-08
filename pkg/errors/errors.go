package apperrors

import (
	"fmt"
	"net/http"
)

// HttpError определяет структуру нашей кастомной ошибки.
// Она позволяет хранить HTTP-статус код и сообщение для пользователя.
type HttpError struct {
	Code    int    `json:"-"`
	Message string `json:"message"`
	Err     error  `json:"-"` // Для внутреннего логирования
}

// Error делает наш HttpError совместимым со стандартным интерфейсом error.
func (e *HttpError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("code: %d, message: %s, internal_error: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("code: %d, message: %s", e.Code, e.Message)
}

// NewHttpError - это публичный конструктор для создания новых, "одноразовых" ошибок прямо в коде.
// Это исправляет ошибку `undefined: apperrors.NewHttpError`.
func NewHttpError(code int, message string, err error) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// --- Предопределенные ошибки для часто используемых случаев ---
var (
	// Ошибки валидации и запроса
	ErrBadRequest = NewHttpError(http.StatusBadRequest, "Неверный запрос", nil)
	ErrValidation = NewHttpError(http.StatusBadRequest, "Ошибка валидации данных", nil)

	// Ошибки аутентификации (Кто ты?)
	ErrUnauthorized       = NewHttpError(http.StatusUnauthorized, "Необходима авторизация", nil)
	ErrEmptyAuthHeader    = NewHttpError(http.StatusUnauthorized, "Заголовок авторизации отсутствует", nil)
	ErrInvalidAuthHeader  = NewHttpError(http.StatusUnauthorized, "Неверный формат заголовка авторизации", nil)
	ErrInvalidCredentials = NewHttpError(http.StatusUnauthorized, "Неверные учётные данные", nil)
	ErrAccountLocked      = NewHttpError(http.StatusLocked, "Аккаунт временно заблокирован из-за множества неудачных попыток входа", nil)
	

	// Ошибки авторизации (Можно ли тебе сюда?)
	ErrForbidden = NewHttpError(http.StatusForbidden, "Доступ запрещен", nil)

	// Ошибки токенов
	ErrInvalidToken      = NewHttpError(http.StatusUnauthorized, "Недопустимый или некорректный токен", nil)
	ErrTokenExpired      = NewHttpError(http.StatusUnauthorized, "Срок действия токена истёк", nil)
	ErrInvalidResetToken = NewHttpError(http.StatusBadRequest, "Невалидный или истекший токен сброса пароля", nil)
	ErrTokenIsNotAccess  = NewHttpError(http.StatusUnauthorized, "Попытка доступа с refresh токеном", nil)

	// Ошибки верификации
	ErrInvalidVerificationCode = NewHttpError(http.StatusBadRequest, "Неверный код верификации", nil)

	// Ошибки, связанные с данными
	ErrNotFound     = NewHttpError(http.StatusNotFound, "Запрашиваемый ресурс не найден", nil)
	ErrUserNotFound = NewHttpError(http.StatusNotFound, "Пользователь не найден", nil)

	// Внутренние ошибки
	ErrInternalServer          = NewHttpError(http.StatusInternalServerError, "Внутренняя ошибка сервера", nil)
	ErrUserIDNotFoundInContext = NewHttpError(http.StatusInternalServerError, "UserID не найден в контексте запроса", nil)
	ErrInvalidSigningMethod    = NewHttpError(http.StatusInternalServerError, "Внутренняя ошибка сервера: неверный метод подписи токена", nil)
	ErrInvalidUserID		   = NewHttpError(http.StatusInternalServerError, "UserID в контексте имеет неверный тип или равен нулю", nil)
)
