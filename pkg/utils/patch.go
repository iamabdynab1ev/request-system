package utils

import (
	"reflect"
	"strings"
	"time"
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

// convertType приводит значение from к типу targetType.
// Решает проблему json.Unmarshal, который парсит числа как float64.
func convertType(from interface{}, targetType reflect.Type) interface{} {
	fromVal := reflect.ValueOf(from)

	// Если типы уже совпадают, возвращаем как есть
	if fromVal.Type() == targetType {
		return from
	}

	// Если можно напрямую сконвертировать
	if fromVal.Type().ConvertibleTo(targetType) {
		return fromVal.Convert(targetType).Interface()
	}

	// НОВОЕ: Обработка строк в time.Time
	if fromVal.Kind() == reflect.String && targetType == reflect.TypeOf(time.Time{}) {
		strVal := fromVal.String()

		// Пробуем RFC3339 (ISO 8601)
		if t, err := time.Parse(time.RFC3339, strVal); err == nil {
			return t
		}

		// Пробуем упрощенный формат
		if t, err := time.Parse("2006-01-02T15:04:05Z07:00", strVal); err == nil {
			return t
		}

		// Пробуем еще один вариант
		if t, err := time.Parse("2006-01-02T15:04:05", strVal); err == nil {
			return t
		}

		// Если не удалось спарсить - возвращаем nil (конвертация не удалась)
		return nil
	}

	// Обработка JSON чисел (которые приходят как float64)
	if fromVal.Kind() == reflect.Float64 {
		floatV := fromVal.Float()
		switch targetType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return int64(floatV) // Возвращаем как int64 (Go потом сам кастанет через Value.Set)
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

	// Если ничего не подошло (например, пытаемся записать строку в число), возвращаем nil
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

	return ""
}
