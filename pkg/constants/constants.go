// pkg/constants/constants.go
package constants

//============== UPLOAD CONTEXTS ==============

// UploadContext определяет тип для контекстов загрузки файлов.
type UploadContext string

const (
	// UploadContextProfilePhoto используется для загрузки фотографий профиля пользователя.
	UploadContextProfilePhoto UploadContext = "profile_photo"
)

// String возвращает строковое представление контекста.
func (uc UploadContext) String() string {
	return string(uc)
}

//============== USER STATUSES ==============

// ID статусов пользователей. Используются для записи в БД, но не для логики.
const (
	UserStatusActiveID uint64 = 10
)

// Коды статусов пользователей. Используются в бизнес-логике для надежности.
const (
	UserStatusActiveCode = "ACTIVE"
)

//============== CACHE KEYS ==============

// Префиксы для ключей в Redis/кеше.
const (
	// Ключ для токена принудительной смены пароля при первом входе.
	// Формат: force_change_token:<token> -> userID
	CacheKeyForceChangeToken = "force_change_token:%s"

	// Ключ для токена сброса пароля, отправленного по email.
	// Формат: reset_email:<token> -> userID
	CacheKeyResetEmail = "reset_email:%s"

	// Ключ для верификационного токена, полученного после ввода SMS-кода.
	// Формат: verify_token_phone:<token> -> userID
	CacheKeyVerifyPhone = "verify_token_phone:%s"

	// Ключ для хранения SMS-кода для сброса пароля.
	// Формат: reset_phone_code:<login> -> code
	CacheKeyResetPhoneCode = "reset_phone_code:%s"

	// Ключ для подсчета попыток сброса пароля (защита от перебора).
	// Формат: reset_attempts:<login> -> count
	CacheKeyResetAttempts = "reset_attempts:%s"

	// Ключ для защиты от спама при запросе сброса пароля.
	// Формат: reset_spam_protect:<login> -> "active"
	CacheKeySpamProtect = "reset_spam_protect:%s"

	// Ключ, указывающий, что аккаунт заблокирован из-за неудачных попыток входа.
	// Формат: lockout:<userID> -> "locked"
	CacheKeyLockout = "lockout:%d"

	// Ключ для подсчета неудачных попыток входа.
	// Формат: login_attempts:<userID> -> count
	CacheKeyLoginAttempts = "login_attempts:%d"
)
