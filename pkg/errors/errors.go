package apperrors

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

type HttpError struct {
	Code    int                    `json:"-"`
	Message string                 `json:"message"`
	Details interface{}            `json:"details,omitempty"`
	Err     error                  `json:"-"`
	Context map[string]interface{} `json:"-"`
}

func (e *HttpError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("code: %d, message: %s, internal: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("code: %d, message: %s", e.Code, e.Message)
}

func NewHttpError(code int, message string, err error, context map[string]interface{}) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Err:     err,
		Context: context,
	}
}

func NewHttpErrorWithDetails(code int, message string, err error, context map[string]interface{}, details interface{}) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Err:     err,
		Context: context,
		Details: details,
	}
}

func NewBadRequestError(message string) *HttpError {
	if message == "" {
		return ErrBadRequest
	}
	return NewHttpError(http.StatusBadRequest, message, nil, nil)
}

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

const (
	sqlStateUniqueViolation           = "23505"
	sqlStateForeignKeyViolation       = "23503"
	sqlStateNotNullViolation          = "23502"
	sqlStateStringDataRightTruncation = "22001"
)

type dbConstraintSpec struct {
	statusCode int
	message    string
}

var exactConstraintSpecs = map[string]dbConstraintSpec{
	"fk_orders_status_id":                       {statusCode: http.StatusBadRequest, message: "Выбранный статус был удалён. Обновите страницу."},
	"fk_orders_priority_id":                     {statusCode: http.StatusBadRequest, message: "Выбранный приоритет был удалён. Обновите страницу."},
	"fk_orders_executor_id":                     {statusCode: http.StatusBadRequest, message: "Выбранный исполнитель не найден. Обновите страницу."},
	"fk_orders_department_id":                   {statusCode: http.StatusBadRequest, message: "Выбранный департамент не найден. Обновите страницу."},
	"fk_orders_branch_id":                       {statusCode: http.StatusBadRequest, message: "Выбранный филиал не найден. Обновите страницу."},
	"fk_orders_otdel_id":                        {statusCode: http.StatusBadRequest, message: "Выбранный отдел не найден. Обновите страницу."},
	"fk_orders_office_id":                       {statusCode: http.StatusBadRequest, message: "Выбранный офис не найден. Обновите страницу."},
	"fk_orders_order_type_id":                   {statusCode: http.StatusBadRequest, message: "Выбранный тип заявки не найден. Обновите страницу."},
	"fk_orders_equipment_id":                    {statusCode: http.StatusBadRequest, message: "Выбранное оборудование не найдено. Обновите страницу."},
	"fk_orders_user_id":                         {statusCode: http.StatusBadRequest, message: "Пользователь не найден. Обновите страницу."},
	"fk_status_id":                              {statusCode: http.StatusBadRequest, message: "Выбранный статус не найден. Обновите страницу."},
	"fk_position_id":                            {statusCode: http.StatusBadRequest, message: "Выбранная должность не найдена. Обновите страницу."},
	"fk_users_position_id":                      {statusCode: http.StatusBadRequest, message: "Выбранная должность не найдена. Обновите страницу."},
	"fk_branches_id":                            {statusCode: http.StatusBadRequest, message: "Выбранный филиал не найден. Обновите страницу."},
	"fk_departments_id":                         {statusCode: http.StatusBadRequest, message: "Выбранный департамент не найден. Обновите страницу."},
	"fk_offices_id":                             {statusCode: http.StatusBadRequest, message: "Выбранный офис не найден. Обновите страницу."},
	"fk_otdels_id":                              {statusCode: http.StatusBadRequest, message: "Выбранный отдел не найден. Обновите страницу."},
	"fk_role_permissions_role_id":               {statusCode: http.StatusBadRequest, message: "Выбранная роль не найдена."},
	"fk_role_permissions_permission_id":         {statusCode: http.StatusBadRequest, message: "Выбранное право не найдено."},
	"fk_user_roles_role_id":                     {statusCode: http.StatusBadRequest, message: "Выбранная роль не найдена."},
	"fk_roles_status_id":                        {statusCode: http.StatusBadRequest, message: "Выбранный статус роли не найден."},
	"fk_equipment_status_id":                    {statusCode: http.StatusBadRequest, message: "Выбранный статус оборудования не найден."},
	"fk_equipment_equipment_type_id":            {statusCode: http.StatusBadRequest, message: "Выбранный тип оборудования не найден."},
	"fk_equipment_branch_id":                    {statusCode: http.StatusBadRequest, message: "Выбранный филиал оборудования не найден."},
	"fk_equipment_office_id":                    {statusCode: http.StatusBadRequest, message: "Выбранный офис оборудования не найден."},
	"branches_name_unique":                      {statusCode: http.StatusBadRequest, message: "Филиал с таким названием уже существует."},
	"departments_name_unique":                   {statusCode: http.StatusBadRequest, message: "Департамент с таким названием уже существует."},
	"equipment_types_name_unique":               {statusCode: http.StatusBadRequest, message: "Тип оборудования с таким названием уже существует."},
	"order_types_name_unique":                   {statusCode: http.StatusBadRequest, message: "Тип заявки с таким названием или кодом уже существует."},
	"order_types_code_unique":                   {statusCode: http.StatusBadRequest, message: "Тип заявки с таким названием или кодом уже существует."},
	"otdels_name_department_id_unique":          {statusCode: http.StatusBadRequest, message: "Отдел с таким названием уже существует в этом департаменте."},
	"permissions_name_key":                      {statusCode: http.StatusBadRequest, message: "Право с таким названием уже существует."},
	"priorities_code_unique":                    {statusCode: http.StatusBadRequest, message: "Приоритет с таким кодом уже существует."},
	"roles_name_key":                            {statusCode: http.StatusBadRequest, message: "Роль с таким названием уже существует."},
	"statuses_code_unique":                      {statusCode: http.StatusBadRequest, message: "Статус с таким кодом уже существует."},
	"positions_name_unique":                     {statusCode: http.StatusBadRequest, message: "Должность с таким названием уже существует."},
	"users_email_key":                           {statusCode: http.StatusBadRequest, message: "Пользователь с таким email уже существует."},
	"users_phone_number_key":                    {statusCode: http.StatusBadRequest, message: "Пользователь с таким номером телефона уже существует."},
	"users_telegram_chat_id_unique":             {statusCode: http.StatusBadRequest, message: "Этот Telegram аккаунт уже привязан к другому пользователю."},
	"unique_order_type_id_in_rules":             {statusCode: http.StatusBadRequest, message: "Правило маршрутизации для этого типа заявки уже существует."},
	"ux_role_permissions_role_id_permission_id": {statusCode: http.StatusBadRequest, message: "Это право уже назначено данной роли."},
	"idx_users_username_unique":                 {statusCode: http.StatusConflict, message: "Этот логин AD уже привязан к другому пользователю."},
}

var prefixConstraintSpecs = map[string]dbConstraintSpec{
	"idx_users_username": {statusCode: http.StatusConflict, message: "Этот логин AD уже привязан к другому пользователю."},
	"statuses_code":      {statusCode: http.StatusBadRequest, message: "Статус с таким кодом уже существует."},
}

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

	var httpErr *HttpError
	if errors.As(err, &httpErr) {
		return err
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if mapped := mapPostgresError(pgErr, err); mapped != nil {
			return mapped
		}
	}

	if mapped := mapDBErrorByText(err); mapped != nil {
		return mapped
	}

	return err
}

func mapPostgresError(pgErr *pgconn.PgError, original error) error {
	if spec, ok := findConstraintSpec(pgErr.ConstraintName); ok {
		return newDBHttpError(spec, original)
	}

	switch pgErr.Code {
	case sqlStateNotNullViolation:
		return newDBHttpError(dbConstraintSpec{statusCode: http.StatusBadRequest, message: "Не заполнено обязательное поле."}, original)
	case sqlStateStringDataRightTruncation:
		return newDBHttpError(dbConstraintSpec{statusCode: http.StatusBadRequest, message: "Одно из полей содержит слишком длинное значение."}, original)
	case sqlStateForeignKeyViolation:
		return newDBHttpError(dbConstraintSpec{statusCode: http.StatusBadRequest, message: "Один из выбранных элементов был удалён. Обновите страницу."}, original)
	case sqlStateUniqueViolation:
		return newDBHttpError(dbConstraintSpec{statusCode: http.StatusBadRequest, message: "Такая запись уже существует."}, original)
	default:
		return nil
	}
}

func findConstraintSpec(constraintName string) (dbConstraintSpec, bool) {
	normalized := strings.ToLower(strings.TrimSpace(constraintName))
	if normalized == "" {
		return dbConstraintSpec{}, false
	}

	if spec, ok := exactConstraintSpecs[normalized]; ok {
		return spec, true
	}

	for prefix, spec := range prefixConstraintSpecs {
		if strings.HasPrefix(normalized, prefix) {
			return spec, true
		}
	}

	return dbConstraintSpec{}, false
}

func mapDBErrorByText(err error) error {
	errMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errMsg, "foreign key constraint"):
		return NewHttpError(http.StatusBadRequest, "Один из выбранных элементов был удалён. Обновите страницу.", err, nil)
	case strings.Contains(errMsg, "unique constraint") || strings.Contains(errMsg, sqlStateUniqueViolation):
		return NewHttpError(http.StatusBadRequest, "Такая запись уже существует.", err, nil)
	case strings.Contains(errMsg, "null value in column") || strings.Contains(errMsg, sqlStateNotNullViolation):
		return NewHttpError(http.StatusBadRequest, "Не заполнено обязательное поле.", err, nil)
	case strings.Contains(errMsg, "value too long") || strings.Contains(errMsg, sqlStateStringDataRightTruncation):
		return NewHttpError(http.StatusBadRequest, "Одно из полей содержит слишком длинное значение.", err, nil)
	case strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no connection"):
		return NewHttpError(http.StatusInternalServerError, "Ошибка соединения с базой данных. Попробуйте позже.", err, nil)
	default:
		return nil
	}
}

func newDBHttpError(spec dbConstraintSpec, err error) *HttpError {
	return NewHttpError(spec.statusCode, spec.message, err, nil)
}
