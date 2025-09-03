// Файл: pkg/customvalidator/validators.go

package customvalidator

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

// RegisterCustomValidations "собирает" все наши кастомные правила валидации
// и регистрирует их в переданном экземпляре валидатора.
func RegisterCustomValidations(v *validator.Validate) error {
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
		return err // Это правило было у вас последним, оставляем его здесь
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
