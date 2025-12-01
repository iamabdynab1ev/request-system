package validation

import (
	"reflect"

	"github.com/aarondl/null/v8"
	"github.com/go-playground/validator/v10"
)

// registerNullTypes учит валидатор "смотреть внутрь" типов null.String, null.Int и т.д.
func registerNullTypes(v *validator.Validate) {
	// Для null.String
	v.RegisterCustomTypeFunc(func(field reflect.Value) interface{} {
		if val, ok := field.Interface().(null.String); ok {
			if val.Valid {
				return val.String // Возвращаем строку
			}
		}
		return nil // Возвращаем nil, чтобы сработал `omitempty`
	}, null.String{})

	// Для null.Int
	v.RegisterCustomTypeFunc(func(field reflect.Value) interface{} {
		if val, ok := field.Interface().(null.Int); ok {
			if val.Valid {
				return val.Int // Возвращаем число
			}
		}
		return nil
	}, null.Int{})

	// Для null.Time
	v.RegisterCustomTypeFunc(func(field reflect.Value) interface{} {
		if val, ok := field.Interface().(null.Time); ok {
			if val.Valid {
				return val.Time // Возвращаем time.Time
			}
		}
		return nil
	}, null.Time{})
}
