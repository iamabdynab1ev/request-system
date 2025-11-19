package seeders

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func seedEquipments(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'equipments'...")

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "TRUNCATE TABLE equipments RESTART IDENTITY CASCADE"); err != nil {
		return err
	}

	// --- ШАГ 1: Загружаем все нужные ID из базы в словари (map) ---
	activeStatusID, err := findIDByCode(ctx, tx, "statuses", "ACTIVE")
	if err != nil {
		return fmt.Errorf("не удалось найти ID статуса 'ACTIVE': %w", err)
	}

	equipmentTypesMap, err := mapAllIDsByName(ctx, tx, "equipment_types")
	if err != nil {
		return fmt.Errorf("ошибка получения ID типов оборудования: %w", err)
	}

	officesMap, err := mapAllIDsByName(ctx, tx, "offices")
	if err != nil {
		return fmt.Errorf("ошибка получения ID офисов: %w", err)
	}

	officeToBranchMap, err := mapOfficeToBranch(ctx, tx)
	if err != nil {
		return fmt.Errorf("ошибка получения связей офис-филиал: %w", err)
	}

	// --- ШАГ 2: Вставляем оборудование, используя ID из словарей ---
	query := `INSERT INTO equipments (name, address, branch_id, office_id, status_id, equipment_type_id) 
			  VALUES ($1, $2, $3, $4, $5, $6)`

	for _, e := range equipmentsData {
		typeID, ok := equipmentTypesMap[e.TypeName]
		if !ok {
			log.Printf("ПРЕДУПРЕЖДЕНИЕ: Тип оборудования '%s' не найден, пропускаем.", e.TypeName)
			continue
		}
		officeID, ok := officesMap[e.OfficeName]
		if !ok {
			log.Printf("ПРЕДУПРЕЖДЕНИЕ: Офис '%s' не найден, пропускаем оборудование '%s'.", e.OfficeName, e.EquipmentName)
			continue
		}
		branchID, ok := officeToBranchMap[officeID]
		if !ok {
			log.Printf("ПРЕДУПРЕЖДЕНИЕ: Не удалось найти филиал для офиса '%s', пропускаем.", e.OfficeName)
			continue
		}

		if _, err := tx.Exec(ctx, query, e.EquipmentName, e.Address, branchID, officeID, activeStatusID, typeID); err != nil {
			log.Printf("Ошибка при вставке оборудования '%s' в офис '%s': %v", e.EquipmentName, e.OfficeName, err)
			// Прерываем при первой ошибке, чтобы понять причину
			return err
		}
	}

	return tx.Commit(ctx)
}

// --- Вспомогательные функции, которые делают код чище ---
func findIDByCode(ctx context.Context, tx pgx.Tx, table, code string) (uint64, error) {
	var id uint64
	query := fmt.Sprintf("SELECT id FROM %s WHERE code = $1", table)
	err := tx.QueryRow(ctx, query, code).Scan(&id)
	return id, err
}

func mapAllIDsByName(ctx context.Context, tx pgx.Tx, table string) (map[string]uint64, error) {
	query := fmt.Sprintf("SELECT id, name FROM %s", table)
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resultMap := make(map[string]uint64)
	for rows.Next() {
		var id uint64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		resultMap[name] = id
	}
	return resultMap, nil
}

func mapOfficeToBranch(ctx context.Context, tx pgx.Tx) (map[uint64]uint64, error) {
	rows, err := tx.Query(ctx, "SELECT id, branch_id FROM offices")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	officeToBranchMap := make(map[uint64]uint64)
	for rows.Next() {
		var officeID, branchID uint64
		if err := rows.Scan(&officeID, &branchID); err != nil {
			return nil, err
		}
		officeToBranchMap[officeID] = branchID
	}
	return officeToBranchMap, nil
}
