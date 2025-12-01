package validation

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

// registerRules регистрирует теги, которые мы используем в struct tags
func registerRules(v *validator.Validate) error {
	if err := v.RegisterValidation("e164_TJ", isTajikPhoneNumber); err != nil {
		return err
	}
	if err := v.RegisterValidation("duration_format", isDurationValid); err != nil {
		return err
	}
	if err := v.RegisterValidation("address_logic", validateAddressWithLocation); err != nil {
		return err
	}
	if err := v.RegisterValidation("custom_email", isGoodEmailFormat); err != nil {
		return err
	}
	return nil
}

// isGoodEmailFormat - проверка email
func isGoodEmailFormat(fl validator.FieldLevel) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(fl.Field().String())
}

// isTajikPhoneNumber - проверка +992...
func isTajikPhoneNumber(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(`^\+992\d{9}$`)
	return re.MatchString(fl.Field().String())
}

// isDurationValid - форматы вида "2h30m"
func isDurationValid(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(`^(\d+h)?(\d+m)?$`)
	s := fl.Field().String()
	return re.MatchString(s) && (strings.Contains(s, "h") || strings.Contains(s, "m"))
}

// validateAddressWithLocation - Сложная логика: если Адрес пуст, должно быть заполнено поле локации (Branch/Otdel)
func validateAddressWithLocation(fl validator.FieldLevel) bool {
	parent := fl.Parent() // Получаем доступ ко всей структуре
	addressField := fl.Field()
	var addressValue string

	// Извлекаем значение адреса (учитывая, что это может быть указатель или строка)
	switch addressField.Kind() {
	case reflect.String:
		addressValue = addressField.String()
	case reflect.Ptr:
		if !addressField.IsNil() {
			addressValue = addressField.Elem().String()
		}
	case reflect.Struct:
		// Поддержка null.String через интерфейс, если вдруг она пришла сюда
		if val, ok := addressField.Interface().(interface{ Value() (interface{}, error) }); ok {
			v, _ := val.Value()
			if str, ok := v.(string); ok {
				addressValue = str
			}
		}
	}

	// Если адрес есть - все хорошо
	if strings.TrimSpace(addressValue) != "" {
		return true
	}

	// Если адреса нет, проверяем наличие хотя бы одного ID локации
	locationFields := []string{"OtdelID", "BranchID", "OfficeID"}
	for _, fieldName := range locationFields {
		field := parent.FieldByName(fieldName)
		// Если поле существует, является указателем и НЕ nil -> локация выбрана -> валидация пройдена
		// Для null.Int нужно проверить поле Valid
		if field.IsValid() {
			if field.Kind() == reflect.Struct { // null.Int
				validField := field.FieldByName("Valid")
				if validField.IsValid() && validField.Bool() {
					return true
				}
			} else if field.Kind() == reflect.Ptr && !field.IsNil() { // *int
				return true
			}
		}
	}

	return false
}
