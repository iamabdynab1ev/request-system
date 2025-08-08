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
func StringToNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func Uint64ToNullInt64(id uint64) sql.NullInt64 {
	if id == 0 {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(id), Valid: true}
}
func NullInt64ToUint64(n sql.NullInt64) uint64 {
	if !n.Valid || n.Int64 == 0 {
		return 0
	}
	return uint64(n.Int64)
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
func StringPtrToNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}
