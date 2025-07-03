package errors

import "fmt"

var (
	// JWT и токены
	ErrInvalidSigningMethod = fmt.Errorf("неверный метод подписи токена")
	ErrInvalidToken         = fmt.Errorf("недопустимый токен")
	ErrTokenExpired         = fmt.Errorf("срок действия токена истёк")
	ErrTokenNotYetValid     = fmt.Errorf("токен ещё не активен")
	ErrTokenNotFound        = fmt.Errorf("токен не найден")
	ErrTokenIsNotRefresh    = fmt.Errorf("токен не является refresh-токеном")

	// Авторизация
	ErrEmptyAuthHeader    = fmt.Errorf("заголовок авторизации отсутствует")
	ErrInvalidAuthHeader  = fmt.Errorf("неверный формат заголовка авторизации")
	ErrInvalidCredentials = fmt.Errorf("неверные учётные данные")
	ErrUnauthorized       = fmt.Errorf("неавторизован")   // <--- ДОБАВЛЕНО
	ErrForbidden          = fmt.Errorf("доступ запрещён") // <--- ДОБАВЛЕНО

		
	// Контекст
	ErrUserIDNotFoundInContext = fmt.Errorf("UserID не найден в контексте запроса")
	ErrInvalidUserID           = fmt.Errorf("недопустимый UserID")

	// Общие
	ErrNotFound   = fmt.Errorf("запись не найдена")
	ErrBadRequest = fmt.Errorf("неверный запрос")
)

// Кастомные типы ошибок
type InvalidInputError struct {
	Message string
}

func (e *InvalidInputError) Error() string { return e.Message }

func NewInvalidInputError(format string, args ...interface{}) error {
	return &InvalidInputError{Message: fmt.Sprintf(format, args...)}
}
