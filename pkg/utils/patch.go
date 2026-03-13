package utils

import (
	"reflect"
	"strings"
	"time"
	"strconv"
)

func SmartUpdate(dst interface{}, changes map[string]interface{}) bool {
	dstVal := reflect.ValueOf(dst).Elem()
	hasChanges := false

	dstType := dstVal.Type()

	for i := 0; i < dstVal.NumField(); i++ {
		fieldInfo := dstType.Field(i)

		// 1. Ищем тег json, чтобы узнать ключ ("otdel_id" и т.д.)
		jsonTag := fieldInfo.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue // Поле скрыто или без тега — пропускаем
		}

		// Убираем ",omitempty" и прочие опции из тега, берем только имя
		jsonKey := ""
		if commaIdx := strings.Index(jsonTag, ","); commaIdx != -1 {
			jsonKey = jsonTag[:commaIdx]
		} else {
			jsonKey = jsonTag
		}

		// 2. Есть ли этот ключ в карте изменений?
		valInMap, existsInMap := changes[jsonKey]
		if !existsInMap {
			// Ключ не прислали — не трогаем поле
			continue
		}

		fieldVal := dstVal.Field(i)
		if !fieldVal.CanSet() {
			continue
		}

		// 3. Если пришел явный NULL (nil)
		if valInMap == nil {
			// Обнуляем поле, если это указатель или интерфейс
			if fieldVal.Kind() == reflect.Ptr || fieldVal.Kind() == reflect.Interface || fieldVal.Kind() == reflect.Slice {
				if !fieldVal.IsNil() {
					fieldVal.Set(reflect.Zero(fieldVal.Type()))
					hasChanges = true
				}
			}
			continue
		}

		// 4. Если пришло значение — пытаемся сконвертировать и записать
		// valInMap имеет тип interface{}, например float64 (стандарт JSON для чисел)

		targetType := fieldVal.Type()
		isPtr := fieldVal.Kind() == reflect.Ptr

		// Если целевое поле - указатель (*uint64, *time.Time), нам нужен тип элемента
		if isPtr {
			targetType = fieldVal.Type().Elem()
		}

		convertedVal := convertType(valInMap, targetType)

		// Если конвертация удалась (результат не nil)
		if convertedVal != nil {
			newValRef := reflect.ValueOf(convertedVal)

			if isPtr {
				// Логика для полей-указателей (*string, *uint64, *time.Time)
				// Создаем новый pointer нужного типа
				newPtr := reflect.New(targetType)
				newPtr.Elem().Set(newValRef)

				// Сравниваем: если старое nil или значения разные — обновляем
				if fieldVal.IsNil() || fieldVal.Elem().Interface() != convertedVal {
					fieldVal.Set(newPtr)
					hasChanges = true
				}
			} else {
				// Логика для обычных полей (string, int)
				if fieldVal.Interface() != convertedVal {
					fieldVal.Set(newValRef)
					hasChanges = true
				}
			}
		}
	}
	return hasChanges
}

func convertType(from interface{}, targetType reflect.Type) interface{} {
	// НОВОЕ: Прямая поддержка *time.Time и time.Time
	if targetType == reflect.TypeOf(time.Time{}) {
		switch v := from.(type) {
		case time.Time:
			return v
		case *time.Time:
			if v != nil {
				return *v
			}
			return nil
		case string:
			// Пробуем RFC3339 (ISO 8601)
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return t
			}
			// Пробуем упрощенный формат
			if t, err := time.Parse("2006-01-02T15:04:05Z07:00", v); err == nil {
				return t
			}
			// Пробуем еще один вариант
			if t, err := time.Parse("2006-01-02T15:04:05", v); err == nil {
				return t
			}
			return nil
		}
		return nil
	}
	
	fromVal := reflect.ValueOf(from)

	// Если типы уже совпадают, возвращаем как есть
	if fromVal.Type() == targetType {
		return from
	}

	// Если можно напрямую сконвертировать
	if fromVal.Type().ConvertibleTo(targetType) {
		return fromVal.Convert(targetType).Interface()
	}

	// Обработка JSON чисел (которые приходят как float64)
	if fromVal.Kind() == reflect.Float64 {
		floatV := fromVal.Float()
		switch targetType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return int64(floatV)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return uint64(floatV)
		}
	}

	// Обработка JSON чисел (если почему-то пришли как int - бывает при других парсерах)
	if fromVal.Kind() == reflect.Int {
		intV := fromVal.Int()
		switch targetType.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return uint64(intV)
		}
	}

	// Если ничего не подошло
	return nil
}

func SafeDeref(ptr *uint64) uint64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func DiffPtr(a, b *uint64) bool {
	if a == nil && b == nil {
		return false
	}
	if a == nil || b == nil {
		return true
	}
	return *a != *b
}

func PtrToString(val *uint64) string {
	if val == nil {
		return ""
	}
	return convertAnyToString(*val)
}

func convertAnyToString(v interface{}) string {
    if s, ok := v.(string); ok {
        return s
    }
    if u, ok := v.(uint64); ok {
        return strconv.FormatUint(u, 10)
    }
    if u, ok := v.(uint32); ok {
        return strconv.FormatUint(uint64(u), 10)
    }
    if f, ok := v.(float64); ok {
        return strconv.FormatUint(uint64(f), 10)
    }
    return ""
}
