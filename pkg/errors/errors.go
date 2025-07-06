package errors

import "errors"

var (
	ErrInvalidSigningMethod = errors.New("неверный метод подписи токена")
	ErrInvalidToken         = errors.New("недопустимый токен")
	ErrTokenExpired         = errors.New("срок действия токена истёк")
	ErrTokenNotYetValid     = errors.New("токен ещё не активен")
	ErrTokenNotFound        = errors.New("токен не найден")
	ErrTokenIsNotRefresh    = errors.New("токен не является refresh-токеном")
	ErrTokenIsNotAccess     = errors.New("токен не является access-токеном")

	ErrValidation = errors.New("ошибка валидации")

	ErrEmptyAuthHeader    = errors.New("заголовок авторизации отсутствует")
	ErrInvalidAuthHeader  = errors.New("неверный формат заголовка авторизации")
	ErrInvalidCredentials = errors.New("неверные учётные данные")

	ErrUserIDNotFoundInContext = errors.New("UserID не найден в контексте запроса")
	ErrInvalidUserID           = errors.New("недопустимый UserID (неверный тип или нулевое значение)")

	ErrNotFound   = errors.New("данные не найдены")
	ErrBadRequest = errors.New("неверный запрос")

	ErrInternalServer          = errors.New("внутренняя ошибка сервера")
	ErrUserNotFound            = errors.New("пользователь не найден")
	ErrInvalidVerificationCode = errors.New("неверный код верификации")
	ErrAccountLocked           = errors.New("счет заблокирован")
)
