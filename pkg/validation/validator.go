package validation

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

type CustomValidator struct {
    validator *validator.Validate
}

func New() *CustomValidator {
    v := validator.New()

    registerNullTypes(v)

    if err := registerRules(v); err != nil {
        panic("ошибка регистрации валидаторов: " + err.Error())
    }

    return &CustomValidator{validator: v}
}
func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		if validationErrs, ok := err.(validator.ValidationErrors); ok {
			if len(validationErrs) > 0 {
				return translateValidationError(validationErrs[0])
			}
		}
		return err
	}
	return nil
}

func translateValidationError(e validator.FieldError) error {
    fieldName := translateFieldName(e.Field())

    switch e.Tag() {
    case "required":
        return fmt.Errorf("поле '%s' обязательно для заполнения", fieldName)
    case "min":
        return fmt.Errorf("поле '%s' должно содержать минимум %s символов", fieldName, e.Param())
    case "max":
        return fmt.Errorf("поле '%s' должно содержать максимум %s символов", fieldName, e.Param())
    case "len":
        return fmt.Errorf("поле '%s' должно содержать ровно %s символов", fieldName, e.Param())
    case "email", "custom_email":
        return fmt.Errorf("поле '%s' должно содержать корректный email", fieldName)
    case "e164_TJ":
        return fmt.Errorf("поле '%s' должно быть в формате +992XXXXXXXXX", fieldName)
    case "duration_format":
        return fmt.Errorf("поле '%s' должно быть в формате '2h30m'", fieldName)
    case "address_logic":
        return fmt.Errorf("укажите адрес или выберите подразделение")
    case "uppercase":
        return fmt.Errorf("поле '%s' должно быть в верхнем регистре", fieldName)
    case "numeric":
        return fmt.Errorf("поле '%s' должно содержать только цифры", fieldName)
    case "gt":
        return fmt.Errorf("поле '%s' должно быть больше %s", fieldName, e.Param())
    case "gte":
        return fmt.Errorf("поле '%s' должно быть не менее %s", fieldName, e.Param())
    case "lt":
        return fmt.Errorf("поле '%s' должно быть меньше %s", fieldName, e.Param())
    case "lte":
        return fmt.Errorf("поле '%s' должно быть не более %s", fieldName, e.Param())
    case "datetime":
        return fmt.Errorf("поле '%s' должно быть в формате дата (например: 2024-01-31)", fieldName)
    case "required_without":
        return fmt.Errorf("поле '%s' обязательно если не указано другое подразделение", fieldName)
    case "dive":
        return fmt.Errorf("поле '%s' содержит недопустимое значение", fieldName)
    case "oneof":
        return fmt.Errorf("поле '%s' содержит недопустимое значение", fieldName)
    case "url":
        return fmt.Errorf("поле '%s' должно содержать корректный URL", fieldName)
    default:
        return fmt.Errorf("поле '%s' не прошло проверку", fieldName)
    }
}

func translateFieldName(field string) string {
    names := map[string]string{
        // Пользователь
        "Fio":          "ФИО",
        "Email":        "Email",
        "PhoneNumber":  "Номер телефона",
        "Password":     "Пароль",
        "NewPassword":  "Новый пароль",
        "Login":        "Логин",
        "Username":     "Имя пользователя",
        "Token":        "Токен",
        "Code":         "Код",
        "RoleIDs":      "Роли",
        "PositionID":   "Должность",
        "StatusID":     "Статус",
        "BranchID":     "Филиал",
        "DepartmentID": "Департамент",
        "OtdelID":      "Отдел",
        "OfficeID":     "Офис",

        // Заявка
        "Name":            "Название",
        "Address":         "Адрес",
        "Comment":         "Комментарий",
        "OrderTypeID":     "Тип заявки",
        "PriorityID":      "Приоритет",
        "ExecutorID":      "Исполнитель",
        "EquipmentID":     "Оборудование",
        "EquipmentTypeID": "Тип оборудования",
        "Duration":        "Срок выполнения",
        "OrderID":         "Заявка",
        "UserID":          "Пользователь",
        "EventType":       "Тип события",

        // Справочники
        "Type":         "Тип",
        "Description":  "Описание",
        "Rate":         "Рейтинг",
        "OpenDate":     "Дата открытия",
        "Path":         "Путь",
        "FileName":     "Имя файла",
        "FilePath":     "Путь к файлу",
        "FileType":     "Тип файла",
        "Message":      "Сообщение",
        "RuleID":       "Правило",
        "RuleName":     "Название правила",
        "PositionType": "Тип должности",
        "PermissionID": "Право",
        "PermissionIDs": "Права",
        "RoleID":       "Роль",
        "ParentID":     "Родительский элемент",
        "DepartmentsID": "Департамент",
    }
    if translated, ok := names[field]; ok {
        return translated
    }
    return field
}
