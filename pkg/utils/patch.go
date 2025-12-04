package utils

import (
	"reflect"
)

func ApplyUpdates(dst interface{}, src interface{}) bool {
	dstVal := reflect.ValueOf(dst).Elem()
	srcVal := reflect.ValueOf(src).Elem()

	hasChanges := false

	for i := 0; i < srcVal.NumField(); i++ {
		srcField := srcVal.Field(i)
		fieldType := srcVal.Type().Field(i)
		fieldName := fieldType.Name

		if srcField.Kind() == reflect.Ptr && srcField.IsNil() {
			continue
		}

		dstField := dstVal.FieldByName(fieldName)
		if !dstField.IsValid() || !dstField.CanSet() {
			continue
		}

		if dstField.Kind() == reflect.Ptr {
			if srcField.Kind() == reflect.Ptr {
				if dstField.IsNil() || dstField.Elem().Interface() != srcField.Elem().Interface() {
					dstField.Set(srcField)
					hasChanges = true
				}
			}
		} else {
			if srcField.Kind() == reflect.Ptr {
				val := srcField.Elem()
				if dstField.Interface() != val.Interface() {
					dstField.Set(val)
					hasChanges = true
				}
			}
		}
	}
	return hasChanges
}
