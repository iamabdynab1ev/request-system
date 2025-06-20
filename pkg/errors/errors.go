package errors

import "errors"

// Здесь будут храниться ВСЕ переиспользуемые ошибки нашего приложения.
var (
	// --- Ошибки, связанные с JWT и токенами ---
	ErrInvalidSigningMethod = errors.New("неверный метод подписи токена")
	ErrInvalidToken         = errors.New("недопустимый токен")
	ErrTokenExpired         = errors.New("срок действия токена истёк")
	ErrTokenNotYetValid     = errors.New("токен ещё не активен")
	ErrTokenNotFound        = errors.New("токен не найден")
	ErrTokenIsNotRefresh    = errors.New("токен не является refresh-токеном")

	// --- Ошибки авторизации и аутентификации ---
	ErrEmptyAuthHeader    = errors.New("заголовок авторизации отсутствует")
	ErrInvalidAuthHeader  = errors.New("неверный формат заголовка авторизации")
	ErrInvalidCredentials = errors.New("неверные учётные данные")

	// --- Ошибки, связанные с данными из контекста ---
	// ↓↓↓ Вот они, недостающие! ↓↓↓
	ErrUserIDNotFoundInContext = errors.New("UserID не найден в контексте запроса")
	ErrInvalidUserID           = errors.New("недопустимый UserID (неверный тип или нулевое значение)")

	// --- Общие ошибки ---
	ErrNotFound   = errors.New("данные не найдены")
	ErrBadRequest = errors.New("неверный запрос")
)
