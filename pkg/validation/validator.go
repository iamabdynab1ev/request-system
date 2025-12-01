package validation

import (
	"github.com/go-playground/validator/v10"
)

// CustomValidator - обертка для использования в Echo
type CustomValidator struct {
	validator *validator.Validate
}

// Validate реализует интерфейс echo.Validator
func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

// New создает и настраивает валидатор
func New() *CustomValidator {
	v := validator.New()

	// 1. Подключаем поддержку null-типов (из файла types_adapter.go)
	registerNullTypes(v)

	// 2. Регистрируем кастомные правила (из файла rules.go)
	// Если правило критично и не зарегистрировалось — паникуем, так как сервер не должен стартовать
	if err := registerRules(v); err != nil {
		panic("ошибка регистрации валидаторов: " + err.Error())
	}

	return &CustomValidator{validator: v}
}
