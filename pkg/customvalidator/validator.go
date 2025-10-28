package customvalidator

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/aarondl/null/v8"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// CustomValidator, Validate() остаются без изменений...
type CustomValidator struct {
	validator *validator.Validate
}

func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

// New - наш исправленный конструктор
func New() echo.Validator {
	validate := validator.New()

	// --- ВОТ ГЛАВНОЕ ИСПРАВЛЕНИЕ ---
	// Регистрируем "обработчики" для null-типов

	// Для null.String
	validate.RegisterCustomTypeFunc(func(field reflect.Value) interface{} {
		if val, ok := field.Interface().(null.String); ok {
			if val.Valid {
				return val.String // Возвращаем обычную строку, если она есть
			}
		}
		return nil // Возвращаем nil, чтобы сработал `omitempty`
	}, null.String{})

	// Для null.Int
	validate.RegisterCustomTypeFunc(func(field reflect.Value) interface{} {
		if val, ok := field.Interface().(null.Int); ok {
			if val.Valid {
				return val.Int // Возвращаем обычное число int64
			}
		}
		return nil
	}, null.Int{})

	// Для null.Time
	validate.RegisterCustomTypeFunc(func(field reflect.Value) interface{} {
		if val, ok := field.Interface().(null.Time); ok {
			if val.Valid {
				return val.Time // Возвращаем time.Time
			}
		}
		return nil
	}, null.Time{})

	// Теперь регистрируем твои кастомные правила
	if err := registerCustomValidations(validate); err != nil {
		panic("не удалось зарегистрировать кастомные валидации: " + err.Error())
	}

	return &CustomValidator{validator: validate}
}

func registerCustomValidations(v *validator.Validate) error {
	// Регистрируем все наши правила по очереди
	if err := v.RegisterValidation("e164_TJ", isTajikPhoneNumber); err != nil {
		return err
	}
	if err := v.RegisterValidation("duration_format", isDurationValid); err != nil {
		return err
	}
	if err := v.RegisterValidation("address_logic", validateAddressWithLocation); err != nil {
		return err
	}
	if err := v.RegisterValidation("email", isGoodEmailFormat); err != nil {
		return err
	}

	return nil
}

// --- Сюда мы перенесли все функции-валидаторы из main.go ---

func isGoodEmailFormat(fl validator.FieldLevel) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(fl.Field().String())
}

func isTajikPhoneNumber(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(`^\+992\d{9}$`)
	return re.MatchString(fl.Field().String())
}

func isDurationValid(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(`^(\d+h)?(\d+m)?$`)
	s := fl.Field().String()
	return re.MatchString(s) && (strings.Contains(s, "h") || strings.Contains(s, "m"))
}

func validateAddressWithLocation(fl validator.FieldLevel) bool {
	parent := fl.Parent()
	addressField := fl.Field()
	var addressValue string

	switch addressField.Kind() {
	case reflect.String:
		addressValue = addressField.String()
	case reflect.Ptr:
		if !addressField.IsNil() {
			addressValue = addressField.Elem().String()
		}
	}

	if strings.TrimSpace(addressValue) != "" {
		return true
	}

	locationFields := []string{"OtdelID", "BranchID", "OfficeID"}
	for _, fieldName := range locationFields {
		field := parent.FieldByName(fieldName)
		if field.IsValid() && field.Kind() == reflect.Ptr && !field.IsNil() {
			return true
		}
	}

	return false
}
