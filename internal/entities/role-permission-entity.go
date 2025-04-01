package entities

import (
    "request-system/pkg/types"
)

type RolePermission struct {
    ID           int `json:"id"`
    RoleID       int `json:"role_id"`
    PermissionID int `json:"permission_id"`

    types.BaseEntity
}
