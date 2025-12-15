// Файл: utils/converters.go
package utils

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/labstack/echo/v4"
)

func NullTimeToString(nt sql.NullTime) *string {
	if !nt.Valid {
		return nil
	}
	formatted := nt.Time.Local().Format("2006-01-02 15:04:05")
	return &formatted
}

func NullTimeToEmptyString(nt sql.NullTime) string {
	if !nt.Valid {
		return ""
	}
	return nt.Time.Local().Format("2006-01-02 15:04:05")
}

func ParseUint64Slice(s []string) ([]uint64, error) {
	if len(s) == 0 {
		return nil, nil
	}

	result := make([]uint64, 0, len(s))
	for _, v := range s {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}

	return result, nil
}

func PtrToNull[T any](ptr *T) sql.Null[T] {
	if ptr == nil {
		return sql.Null[T]{Valid: false}
	}
	return sql.Null[T]{V: *ptr, Valid: true}
}

func NullToPtr[T any](n sql.Null[T]) *T {
	if n.Valid {
		return &n.V
	}
	return nil
}

func NullToValue[T any](n sql.Null[T]) T {
	if n.Valid {
		return n.V
	}
	var zero T
	return zero
}

func ConvertNullIntToUintPtr(n sql.Null[int64]) *uint64 {
	if n.Valid {
		v := uint64(n.V)
		return &v
	}
	return nil
}

func FormatNullTime(n sql.Null[time.Time]) string {
	if n.Valid && !n.V.IsZero() {
		return n.V.Local().Format("2006-01-02 15:04:05")
	}
	return ""
}

func Ctx(c echo.Context, seconds int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request().Context(), time.Duration(seconds)*time.Second)
}

func NullIntToUintPtr(n null.Int) *uint64 {
	if n.Valid {
		val := uint64(n.Int)
		return &val
	}
	return nil
}

func NullTimeToStrPtr(n null.Time) *string {
	if n.Valid {
		val := n.Time.Format(time.RFC3339)
		return &val
	}
	return nil
}

func WasFieldSent(jsonKey string, rawRequestBody []byte) bool {
	var sentFields map[string]interface{}
	// Игнорируем ошибку, т.к. json всегда должен быть валидным на этом этапе
	_ = json.Unmarshal(rawRequestBody, &sentFields)
	_, wasSent := sentFields[jsonKey]
	return wasSent
}

// ParseTime пытается распарсить строку формата RFC3339 в *time.Time.
func ParseTime(timeStr *string) *time.Time {
	if timeStr == nil || *timeStr == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, *timeStr)
	if err != nil {
		return nil
	}
	return &parsed
}

func Uint64PtrToNullInt(ptr *uint64) null.Int {
	if ptr == nil {
		return null.IntFromPtr(nil)
	}
	val := int(*ptr)
	return null.IntFrom(val)
}

// Переименовываем, чтобы сделать функцию экспортируемой

type NullableInt struct {
	null.Int
}

func (ni *NullableInt) UnmarshalJSON(b []byte) error {
	if string(b) == "{}" {
		ni.Valid = false
		return nil
	}
	return ni.Int.UnmarshalJSON(b)
}

type NullableDuration struct {
	null.String
}

func (ns *NullableDuration) UnmarshalJSON(b []byte) error {
	if string(b) == "{}" {
		ns.Valid = false
		return nil
	}
	return ns.String.UnmarshalJSON(b)
}

func NullIntToUint64Ptr(n null.Int) *uint64 {
	if n.Valid {
		v := uint64(n.Int)
		return &v
	}
	return nil
}

func NullStringToStrPtr(n null.String) *string {
	if n.Valid {
		return &n.String
	}
	return nil
}

func Uint64ToNull(v uint64) null.Int64 {
	return null.Int64{
		Int64: int64(v),
		Valid: true,
	}
}

// Uint64PtrToNull — конвертирует *uint64 в null.Int64
func Uint64PtrToNull(v *uint64) null.Int64 {
	if v == nil {
		return null.Int64{
			Valid: false,
		}
	}
	return null.Int64{
		Int64: int64(*v),
		Valid: true,
	}
}

// NullInt64ToUint64Ptr — конвертирует null.Int64 в *uint64
func NullInt64ToUint64Ptr(n null.Int64) *uint64 {
	if n.Valid {
		v := uint64(n.Int64)
		return &v
	}
	return nil
}

func StrPtrToNull(ptr *string) null.String {
	if ptr == nil {
		return null.String{}
	}
	return null.String{String: *ptr, Valid: true}
}

func TimeToNull(t *time.Time) null.Time {
	if t == nil {
		return null.Time{}
	}
	return null.Time{Time: *t, Valid: true}
}

func TimeToNullString(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.Format(time.RFC3339), Valid: true}
}

// Change sql.Null[string] to sql.NullString and use String field
func Uint64ToNullString(u uint64) sql.NullString {
	return sql.NullString{String: fmt.Sprintf("%d", u), Valid: true}
}

// Change sql.Null[string] to sql.NullString
func Uint64PtrToNullString(p *uint64) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return Uint64ToNullString(*p)
}

// New function to convert *string to sql.NullString (fixes mismatch with null.String from StrPtrToNull)
func StrPtrToSQLNullString(ptr *string) sql.NullString {
	if ptr == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *ptr, Valid: true}
}

func NullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func FormatTimePtr(t *time.Time) string {
	if t != nil {
		return t.Format(time.RFC3339)
	}
	return ""
}

func Uint32PtrToNullInt(ptr *uint32) null.Int {
	if ptr == nil {
		return null.IntFromPtr(nil)
	}
	val := int(*ptr)
	return null.IntFrom(val)
}

func StringToUint64(s string) uint64 {
	val, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0 // Возвращаем 0, если строка не является числом
	}
	return val
}

func StringToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// TimeToPtr конвертирует time.Time в *time.Time, возвращая nil для нулевого времени.
func TimeToPtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func GetStringFromPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
