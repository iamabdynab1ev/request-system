// Файл: internal/dto/claims_dto.go
package dto

// UserClaims хранит полную информацию о пользователе, которая будет доступна в контексте запроса.
type UserClaims struct {
	UserID      uint64
	RoleID      uint64
	Permissions []string // Список кодов привилегий, например ["order:create", "order:read_all"]
}