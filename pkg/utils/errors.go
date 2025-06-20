package utils

import (
	"errors"
	"net/http"
)

// Ошибки — экспортируемые и переиспользуемые
var (
	// JWT/Token
	ErrInvalidSigningMethod = errors.New("неверный метод подписи токена")
	ErrInvalidToken         = errors.New("недопустимый токен")
	ErrTokenExpired         = errors.New("срок действия токена истёк")
	ErrTokenNotYetValid     = errors.New("токен ещё не активен")
	ErrTokenNotFound        = errors.New("токен не найден")
	ErrTokenIsNotRefresh    = errors.New("токен не является refresh-токеном")

	// Авторизация
	ErrEmptyAuthHeader    = errors.New("заголовок авторизации отсутствует")
	ErrInvalidAuthHeader  = errors.New("неверный формат заголовка авторизации")
	ErrInvalidCredentials = errors.New("неверные учётные данные")

	// Общие
	ErrorNotFound   = errors.New("данные не найдены")
	ErrorBadRequest = errors.New("неверный запрос")
)

// Сопоставление ошибок с HTTP-кодами
var ErrorStatusCode = map[error]int{
	// Авторизация и токены
	ErrInvalidSigningMethod: http.StatusUnauthorized, // 401
	ErrInvalidToken:         http.StatusUnauthorized,
	ErrTokenExpired:         http.StatusUnauthorized,
	ErrTokenNotYetValid:     http.StatusUnauthorized,
	ErrTokenNotFound:        http.StatusUnauthorized,
	ErrTokenIsNotRefresh:    http.StatusUnauthorized,

	ErrEmptyAuthHeader:    http.StatusUnauthorized,
	ErrInvalidAuthHeader:  http.StatusUnauthorized,
	ErrInvalidCredentials: http.StatusUnauthorized,

	// Общие
	ErrorNotFound:   http.StatusNotFound,   // 404
	ErrorBadRequest: http.StatusBadRequest, // 400
}

// Получить код статуса по ошибке
func GetStatusCode(err error) int {
	if code, ok := ErrorStatusCode[err]; ok {
		return code
	}
	return http.StatusInternalServerError // 500 по умолчанию
}
