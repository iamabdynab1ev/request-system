package apperrors

import (
	"fmt"
	"net/http"
	"strings"
)

// HttpError - структура для кастомных HTTP-ошибок.
type HttpError struct {
	Code    int                    `json:"-"`
	Message string                 `json:"message"`           // Сообщение для пользователя
	Details interface{}            `json:"details,omitempty"` // Поле для доп. данных в JSON-ответе
	Err     error                  `json:"-"`                 // Внутреннее сообщение для логов
	Context map[string]interface{} `json:"-"`                 // Доп. данные для логов
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
		// Details по умолчанию nil, устанавливается при необходимости
	}
}

// NewHttpErrorWithDetails - новый конструктор для ошибок с деталями в ответе.
func NewHttpErrorWithDetails(code int, message string, err error, context map[string]interface{}, details interface{}) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Err:     err,
		Context: context,
		Details: details, // Устанавливаем details
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
	ErrUserDisabled         = NewHttpError(http.StatusForbidden, "Аккаунт неактивен", nil, nil)
	ErrTokenIsNotAccess     = NewHttpError(http.StatusUnauthorized, "Токен не является access токеном", nil, nil)
	ErrInvalidAuthHeader    = NewHttpError(http.StatusUnauthorized, "Недействительный заголовок авторизации", nil, nil)
	ErrEmptyAuthHeader      = NewHttpError(http.StatusUnauthorized, "Отсутствует заголовок авторизации", nil, nil)

	ErrChangePasswordWithToken = NewHttpErrorWithDetails(http.StatusAccepted, "Требуется смена пароля", nil, nil, nil)
	ErrNoChanges               = NewHttpError(http.StatusBadRequest, "Нет изменений в запросе", nil, nil)
)

func IsNotFound(err error) bool {
	e, ok := err.(*HttpError)
	return ok && e.Code == http.StatusNotFound
}

func NewInternalError(msg string) *HttpError {
	return NewHttpError(http.StatusInternalServerError, msg, nil, nil)
}
func WrapDBError(err error) error {
    if err == nil {
        return nil
    }
    errMsg := err.Error()

    // ===== ВНЕШНИЕ КЛЮЧИ — ЗАЯВКИ =====
    if strings.Contains(errMsg, "fk_orders_status_id") {
        return NewBadRequestError("Выбранный статус был удалён. Обновите страницу.")
    }
    if strings.Contains(errMsg, "fk_orders_priority_id") {
        return NewBadRequestError("Выбранный приоритет был удалён. Обновите страницу.")
    }
    if strings.Contains(errMsg, "fk_orders_executor_id") {
        return NewBadRequestError("Выбранный исполнитель не найден. Обновите страницу.")
    }
    if strings.Contains(errMsg, "fk_orders_department_id") {
        return NewBadRequestError("Выбранный департамент не найден. Обновите страницу.")
    }
    if strings.Contains(errMsg, "fk_orders_branch_id") {
        return NewBadRequestError("Выбранный филиал не найден. Обновите страницу.")
    }
    if strings.Contains(errMsg, "fk_orders_otdel_id") {
        return NewBadRequestError("Выбранный отдел не найден. Обновите страницу.")
    }
    if strings.Contains(errMsg, "fk_orders_office_id") {
        return NewBadRequestError("Выбранный офис не найден. Обновите страницу.")
    }
    if strings.Contains(errMsg, "fk_orders_order_type_id") {
        return NewBadRequestError("Выбранный тип заявки не найден. Обновите страницу.")
    }
    if strings.Contains(errMsg, "fk_orders_equipment_id") {
        return NewBadRequestError("Выбранное оборудование не найдено. Обновите страницу.")
    }
    if strings.Contains(errMsg, "fk_orders_user_id") {
        return NewBadRequestError("Пользователь не найден. Обновите страницу.")
    }

    // ===== ВНЕШНИЕ КЛЮЧИ — ПОЛЬЗОВАТЕЛИ =====
    if strings.Contains(errMsg, "fk_status_id") {
        return NewBadRequestError("Выбранный статус не найден. Обновите страницу.")
    }
    if strings.Contains(errMsg, "fk_position_id") || strings.Contains(errMsg, "fk_users_position_id") {
        return NewBadRequestError("Выбранная должность не найдена. Обновите страницу.")
    }
    if strings.Contains(errMsg, "fk_branches_id") {
        return NewBadRequestError("Выбранный филиал не найден. Обновите страницу.")
    }
    if strings.Contains(errMsg, "fk_departments_id") {
        return NewBadRequestError("Выбранный департамент не найден. Обновите страницу.")
    }
    if strings.Contains(errMsg, "fk_offices_id") {
        return NewBadRequestError("Выбранный офис не найден. Обновите страницу.")
    }
    if strings.Contains(errMsg, "fk_otdels_id") {
        return NewBadRequestError("Выбранный отдел не найден. Обновите страницу.")
    }

    // ===== ВНЕШНИЕ КЛЮЧИ — РОЛИ И ПРАВА =====
    if strings.Contains(errMsg, "fk_role_permissions_role_id") {
        return NewBadRequestError("Выбранная роль не найдена.")
    }
    if strings.Contains(errMsg, "fk_role_permissions_permission_id") {
        return NewBadRequestError("Выбранное право не найдено.")
    }
    if strings.Contains(errMsg, "fk_user_roles_role_id") {
        return NewBadRequestError("Выбранная роль не найдена.")
    }
    if strings.Contains(errMsg, "fk_roles_status_id") {
        return NewBadRequestError("Выбранный статус роли не найден.")
    }

    // ===== ВНЕШНИЕ КЛЮЧИ — ОБОРУДОВАНИЕ =====
    if strings.Contains(errMsg, "fk_equipment_status_id") {
        return NewBadRequestError("Выбранный статус оборудования не найден.")
    }
    if strings.Contains(errMsg, "fk_equipment_equipment_type_id") {
        return NewBadRequestError("Выбранный тип оборудования не найден.")
    }
    if strings.Contains(errMsg, "fk_equipment_branch_id") {
        return NewBadRequestError("Выбранный филиал оборудования не найден.")
    }
    if strings.Contains(errMsg, "fk_equipment_office_id") {
        return NewBadRequestError("Выбранный офис оборудования не найден.")
    }

    // ===== УНИКАЛЬНОСТЬ =====
    if strings.Contains(errMsg, "branches_name_unique") {
        return NewBadRequestError("Филиал с таким названием уже существует.")
    }
    if strings.Contains(errMsg, "departments_name_unique") {
        return NewBadRequestError("Департамент с таким названием уже существует.")
    }
    if strings.Contains(errMsg, "equipment_types_name_unique") {
        return NewBadRequestError("Тип оборудования с таким названием уже существует.")
    }
    if strings.Contains(errMsg, "order_types_name_unique") || strings.Contains(errMsg, "order_types_code_unique") {
        return NewBadRequestError("Тип заявки с таким названием или кодом уже существует.")
    }
    if strings.Contains(errMsg, "otdels_name_department_id_unique") {
        return NewBadRequestError("Отдел с таким названием уже существует в этом департаменте.")
    }
    if strings.Contains(errMsg, "permissions_name_key") {
        return NewBadRequestError("Право с таким названием уже существует.")
    }
    if strings.Contains(errMsg, "priorities_code_unique") {
        return NewBadRequestError("Приоритет с таким кодом уже существует.")
    }
    if strings.Contains(errMsg, "roles_name_key") {
        return NewBadRequestError("Роль с таким названием уже существует.")
    }
    if strings.Contains(errMsg, "statuses_code_unique") {
        return NewBadRequestError("Статус с таким кодом уже существует.")
    }
    if strings.Contains(errMsg, "positions_name_unique") {
        return NewBadRequestError("Должность с таким названием уже существует.")
    }
    if strings.Contains(errMsg, "users_email_key") {
        return NewBadRequestError("Пользователь с таким email уже существует.")
    }
    if strings.Contains(errMsg, "users_phone_number_key") {
        return NewBadRequestError("Пользователь с таким номером телефона уже существует.")
    }
    if strings.Contains(errMsg, "users_telegram_chat_id_unique") {
        return NewBadRequestError("Этот Telegram аккаунт уже привязан к другому пользователю.")
    }
    if strings.Contains(errMsg, "unique_order_type_id_in_rules") {
        return NewBadRequestError("Правило маршрутизации для этого типа заявки уже существует.")
    }
    if strings.Contains(errMsg, "ux_role_permissions_role_id_permission_id") {
        return NewBadRequestError("Это право уже назначено данной роли.")
    }

    // ===== ОБЩИЕ ОШИБКИ БД =====
    if strings.Contains(errMsg, "foreign key constraint") {
        return NewBadRequestError("Один из выбранных элементов был удалён. Обновите страницу.")
    }
    if strings.Contains(errMsg, "unique constraint") || strings.Contains(errMsg, "23505") {
        return NewBadRequestError("Такая запись уже существует.")
    }
    if strings.Contains(errMsg, "null value in column") || strings.Contains(errMsg, "23502") {
        return NewBadRequestError("Не заполнено обязательное поле.")
    }
    if strings.Contains(errMsg, "value too long") || strings.Contains(errMsg, "22001") {
        return NewBadRequestError("Одно из полей содержит слишком длинное значение.")
    }
    if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no connection") {
        return NewInternalError("Ошибка соединения с базой данных. Попробуйте позже.")
    }

    return err
}
