package utils

import (
	"context"
	"database/sql"
	"time"

	"github.com/labstack/echo/v4"
)

func Ctx(c echo.Context, seconds int) context.Context {
	newCtx, cancel := context.WithTimeout(c.Request().Context(), time.Duration(seconds)*time.Second)

	go func() {
		<-newCtx.Done()
		cancel()
	}()

	return newCtx
}

func Uint64ToNullInt64(id uint64) sql.NullInt64 {
	if id == 0 {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(id), Valid: true}
}

func NullTimeToString(nt sql.NullTime) *string {
	if !nt.Valid {
		return nil
	}
	formatted := nt.Time.Local().Format("2006-01-02 15:04:05")

	return &formatted
}

func NullStringToString(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

func NullTimeToEmptyString(nt sql.NullTime) string {
	if !nt.Valid {
		return ""
	}
	return nt.Time.Local().Format("2006-01-02 15:04:05")
}

func AreUint64PointersEqual(a, b *uint64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func StringPointerToNullString(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

// StringToNullString конвертирует string в sql.NullString
func StringToNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// <<<--- НАЧАЛО: НОВЫЕ ХЕЛПЕРЫ ДЛЯ ЧИСЕЛ ---

// NullInt64ToUint64 конвертирует sql.NullInt64 в uint64.
// Полезно, когда из БД приходит ID, который может быть NULL.
func NullInt64ToUint64(n sql.NullInt64) uint64 {
	if !n.Valid {
		return 0
	}
	return uint64(n.Int64)
}

// NullInt32ToInt конвертирует sql.NullInt32 в int.
// Эта функция решает вашу вторую ошибку компиляции.
func NullInt32ToInt(n sql.NullInt32) int {
	if !n.Valid {
		return 0 // или другое значение по умолчанию
	}
	return int(n.Int32)
}
