package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func seedRolePermissions(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'role_permissions'...")
	var superAdminRoleID uint64
	err := db.QueryRow(ctx, "SELECT id FROM roles WHERE name = 'Super Admin' LIMIT 1").Scan(&superAdminRoleID)
	if err != nil {
		return err
	}
	var superuserPermissionID uint64
	err = db.QueryRow(ctx, "SELECT id FROM permissions WHERE name = 'superuser' LIMIT 1").Scan(&superuserPermissionID)
	if err != nil {
		return err
	}

	query := `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT (role_id, permission_id) DO NOTHING;`
	if _, err := db.Exec(ctx, query, superAdminRoleID, superuserPermissionID); err != nil {
		return err
	}
	return nil
}
