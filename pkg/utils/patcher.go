// Файл: utils/patcher.go
package utils

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/aarondl/null/v8"
)

// ApplyPatchFinal - ФИНАЛЬНАЯ, РАБОЧАЯ ВЕРСИЯ
func ApplyPatchFinal(entity interface{}, patchDTO interface{}, rawRequestBody []byte) error {
	var sentFields map[string]interface{}
	if err := json.Unmarshal(rawRequestBody, &sentFields); err != nil {
		return err
	}

	entityValue := reflect.ValueOf(entity).Elem()
	patchDTOValue := reflect.ValueOf(patchDTO)
	if patchDTOValue.Kind() == reflect.Ptr {
		patchDTOValue = patchDTOValue.Elem()
	}
	entityType := entityValue.Type()

	for i := 0; i < patchDTOValue.NumField(); i++ {
		patchField := patchDTOValue.Field(i)
		patchFieldType := patchDTOValue.Type().Field(i)
		jsonFieldName := strings.Split(patchFieldType.Tag.Get("json"), ",")[0]

		if _, fieldWasSent := sentFields[jsonFieldName]; !fieldWasSent {
			continue
		}

		entityFieldName := patchFieldType.Name
		if _, found := entityType.FieldByName(entityFieldName); !found {
			continue
		}

		entityFieldValue := entityValue.FieldByName(entityFieldName)
		if !entityFieldValue.IsValid() || !entityFieldValue.CanSet() {
			continue
		}

		if sentFields[jsonFieldName] == nil {
			entityFieldValue.Set(reflect.Zero(entityFieldValue.Type()))
			continue
		}

		// --- УНИВЕРСАЛЬНЫЙ SWITCH ---
		switch patchValue := patchField.Interface().(type) {

		// Обработка *string из DTO
		case *string:
			if patchValue != nil {
				if entityFieldValue.Kind() == reflect.String {
					entityFieldValue.SetString(*patchValue)
				} else if entityFieldValue.Type() == reflect.TypeOf(new(string)) {
					entityFieldValue.Set(reflect.ValueOf(patchValue))
				}
			}

		// Обработка *int из DTO
		case *int:
			if patchValue != nil {
				if entityFieldValue.Kind() == reflect.Int {
					entityFieldValue.SetInt(int64(*patchValue))
				} else if entityFieldValue.Type() == reflect.TypeOf(new(int)) {
					entityFieldValue.Set(reflect.ValueOf(patchValue))
				}
			}

		// Обработка null.String из DTO
		case null.String:
			if entityFieldValue.Type() == reflect.TypeOf(new(string)) {
				if patchValue.Valid {
					entityFieldValue.Set(reflect.ValueOf(&patchValue.String))
				} else {
					entityFieldValue.Set(reflect.Zero(entityFieldValue.Type()))
				}
			} else if entityFieldValue.Kind() == reflect.String {
				if patchValue.Valid {
					entityFieldValue.SetString(patchValue.String)
				}
			}

		// Обработка null.Int из DTO
		case null.Int:
			targetType := entityFieldValue.Type()
			if !patchValue.Valid {
				entityFieldValue.Set(reflect.Zero(targetType))
				continue
			}
			val64 := patchValue.Int
			switch targetType.Kind() {
			case reflect.Ptr:
				switch targetType {
				case reflect.TypeOf(new(int)):
					val := int(val64)
					entityFieldValue.Set(reflect.ValueOf(&val))
				case reflect.TypeOf(new(uint64)):
					val := uint64(val64)
					entityFieldValue.Set(reflect.ValueOf(&val))
				}
			case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64:
				entityFieldValue.Set(reflect.ValueOf(val64).Convert(targetType))
			}

		// --- ДОБАВЛЕН БЛОК ДЛЯ НАШЕГО НОВОГО ТИПА ---
		case NullableInt:
			targetType := entityFieldValue.Type()
			if !patchValue.Int.Valid {
				entityFieldValue.Set(reflect.Zero(targetType))
				continue
			}
			val64 := patchValue.Int.Int
			switch targetType.Kind() {
			case reflect.Ptr:
				switch targetType {
				case reflect.TypeOf(new(int)):
					val := int(val64)
					entityFieldValue.Set(reflect.ValueOf(&val))
				case reflect.TypeOf(new(uint64)):
					val := uint64(val64)
					entityFieldValue.Set(reflect.ValueOf(&val))
				}
			case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64:
				entityFieldValue.Set(reflect.ValueOf(val64).Convert(targetType))
			}
		case *bool:
			if patchValue != nil {
				if entityFieldValue.Kind() == reflect.Bool {
					entityFieldValue.SetBool(*patchValue)
				} else if entityFieldValue.Type() == reflect.TypeOf(new(bool)) {
					entityFieldValue.Set(reflect.ValueOf(patchValue))
				}
			}

		// Обработка null.Bool (если используешь)
		case null.Bool:
			if entityFieldValue.Type() == reflect.TypeOf(new(bool)) {
				if patchValue.Valid {
					entityFieldValue.Set(reflect.ValueOf(&patchValue.Bool))
				} else {
					entityFieldValue.Set(reflect.Zero(entityFieldValue.Type()))
				}
			} else if entityFieldValue.Kind() == reflect.Bool {
				if patchValue.Valid {
					entityFieldValue.SetBool(patchValue.Bool)
				}
			}

		}
	}
	return nil
}
