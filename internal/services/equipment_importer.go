package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"
)

type dbEnt struct { ID uint64; Name string }

type EquipImportService struct {
	db *pgxpool.Pool
}

func NewEquipImportService(db *pgxpool.Pool) *EquipImportService {
	return &EquipImportService{db: db}
}

func (s *EquipImportService) ImportAtms(path string) error      { return s.masterImport(path, "Банкомат") }
func (s *EquipImportService) ImportPos(path string) error       { return s.masterImport(path, "Пос-терминал") }
func (s *EquipImportService) ImportTerminals(path string) error { return s.masterImport(path, "ТЕРМИНАЛ_СМАРТ") }

func (s *EquipImportService) masterImport(filePath string, targetType string) error {
	f, err := excelize.OpenFile(filePath)
	if err != nil { return fmt.Errorf("ошибка открытия файла: %w", err) }
	defer f.Close()

	ctx := context.Background()
	branchData := s.getRawEntities(ctx, "branches")
	officeData := s.getRawEntities(ctx, "offices")
	statusID := s.getOrCreate(ctx, "statuses", "ACTIVE", "code")

	// Справочник типов
	vnutrTypeID   := s.getOrCreate(ctx, "equipment_types", "Внутренний терминал", "name")
	vneshTypeID   := s.getOrCreate(ctx, "equipment_types", "Внешний терминал", "name")
	cashTypeID    := s.getOrCreate(ctx, "equipment_types", "Терминал Cash-in/out", "name")
	defaultTypeID := s.getOrCreate(ctx, "equipment_types", targetType, "name")

	success, errors, updated := 0, 0, 0
	
	// Список имен, которые мы нашли в Excel (чтобы не удалять их)
	processedNames := []string{}
	// Список типов, которые участвуют в этом импорте
	touchedTypesMap := make(map[uint64]bool)

	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil { continue }
		
		fmt.Printf("🔍 Анализирую лист: %s\n", sheet)

		var bIdx, nIdx, oIdx, aIdx, kIdx = -1, -1, -1, -1, -1
		var headerFoundRow = -1

		for rIdx, row := range rows {
			rowStr := strings.ToLower(strings.Join(row, "|"))
			if strings.Contains(rowStr, "филиал") || strings.Contains(rowStr, "номер") || strings.Contains(rowStr, "№") {
				for cIdx, colName := range row {
					cLower := strings.ToLower(strings.TrimSpace(colName))
					if strings.Contains(cLower, "филиал") { bIdx = cIdx }
					if strings.Contains(cLower, "номер") || strings.Contains(cLower, "№") { nIdx = cIdx }
					if strings.Contains(cLower, "цбо") || strings.Contains(cLower, "территор") || strings.Contains(cLower, "учр") { oIdx = cIdx }
					if strings.Contains(cLower, "адрес") { aIdx = cIdx }
					if strings.Contains(cLower, "вид") || strings.Contains(cLower, "тип") { kIdx = cIdx }
				}
				if nIdx != -1 {
					headerFoundRow = rIdx
					break
				}
			}
		}

		if headerFoundRow == -1 { continue }

		for i := headerFoundRow + 1; i < len(rows); i++ {
			row := rows[i]
			if len(row) < 2 { continue }

			name := s.safeGet(row, nIdx)
			if name == "" || s.isTrash(name) { continue }

			// Добавляем имя в список обработанных
			processedNames = append(processedNames, name)

			branchName := s.safeGet(row, bIdx)
			officeName := s.safeGet(row, oIdx)
			address    := s.safeGet(row, aIdx)
			vidText    := strings.ToLower(s.safeGet(row, kIdx))

			finalTypeID := defaultTypeID
			if targetType == "ТЕРМИНАЛ_СМАРТ" || kIdx != -1 {
				if strings.Contains(vidText, "внеш") {
					finalTypeID = vneshTypeID
				} else if strings.Contains(vidText, "внутр") {
					finalTypeID = vnutrTypeID
				} else if strings.Contains(vidText, "cash") {
					finalTypeID = cashTypeID
				}
			}
			touchedTypesMap[finalTypeID] = true

			if address == "" {
				if officeName != "" { address = officeName } else if branchName != "" { address = branchName } else { address = "-" }
			}

			bID := s.fuzzyFind(branchName, branchData)
			oID := s.fuzzyFind(officeName, officeData)

			var dbBID interface{} = nil
			if bID > 0 { dbBID = bID }
			var dbOID interface{} = nil
			if oID > 0 { dbOID = oID }

			query := `
                INSERT INTO equipments (name, address, branch_id, office_id, status_id, equipment_type_id, updated_at)
                VALUES ($1, $2, $3, $4, $5, $6, NOW())
                ON CONFLICT (name) 
                DO UPDATE SET 
                    address = COALESCE(NULLIF(EXCLUDED.address, '-'), equipments.address), 
                    branch_id = COALESCE($3, equipments.branch_id), 
                    office_id = COALESCE($4, equipments.office_id), 
                    equipment_type_id = EXCLUDED.equipment_type_id,
                    updated_at = NOW()
                RETURNING (xmax = 0) AS is_insert`
			
			var isInsert bool
			err = s.db.QueryRow(ctx, query, name, address, dbBID, dbOID, statusID, finalTypeID).Scan(&isInsert)

			if err != nil {
				fmt.Printf("❌ Стр %d: [%s] Ошибка БД: %v\n", i+1, name, err)
				errors++
			} else {
				if isInsert { success++ } else { updated++ }
			}
		}
	}

	// === ЛОГИКА УДАЛЕНИЯ ЛИШНИХ ===
	if len(processedNames) > 0 {
		typeIDs := []uint64{}
		for tID := range touchedTypesMap { typeIDs = append(typeIDs, tID) }

		// Удаляем те записи, которые относятся к текущим типам оборудования, 
		// но не встретились в Excel файле.
		delQuery := `DELETE FROM equipments WHERE equipment_type_id = ANY($1) AND name != ALL($2)`
		cmdTag, delErr := s.db.Exec(ctx, delQuery, typeIDs, processedNames)
		if delErr != nil {
			fmt.Printf("⚠️ Ошибка при удалении устаревших записей: %v\n", delErr)
		} else {
			deletedCount := cmdTag.RowsAffected()
			if deletedCount > 0 {
				fmt.Printf("🧹 Очистка: удалено %d записей, отсутствующих в Excel.\n", deletedCount)
			}
		}
	}

	fmt.Printf("\n🏁 ИТОГ: Новых: %d | Обновлено: %d | Удалено: %d | Ошибок: %d\n", 
		success, updated, (len(processedNames) - success - updated), errors) // цифра в итого просто примерная для лога
	return nil
}

func (s *EquipImportService) fuzzyFind(excelName string, dbItems []dbEnt) uint64 {
	excelName = strings.ToLower(strings.TrimSpace(excelName))
	if excelName == "" { return 0 }
	cleanExcel := cleanString(excelName)
	for _, item := range dbItems {
		cleanDB := cleanString(item.Name)
		if cleanDB == cleanExcel || strings.Contains(cleanDB, cleanExcel) || strings.Contains(cleanExcel, cleanDB) {
			return item.ID
		}
	}
	return 0
}

func cleanString(in string) string {
	replacer := strings.NewReplacer(
		"филиал", "", "цбо", "", "мхмх", "", "г.", "", 
		"\"", "", "«", "", "»", "", 
		" ", "", ".", "", "-", "", 
		"район", "", "обслуживания", "",
	)
	return strings.TrimSpace(replacer.Replace(strings.ToLower(in)))
}

func (s *EquipImportService) isTrash(val string) bool {
	v := strings.ToLower(strings.TrimSpace(val))
	if v == "" || strings.Contains(v, "итого") || strings.Contains(v, "всего") { return true }
	return false
}

func (s *EquipImportService) getRawEntities(ctx context.Context, table string) []dbEnt {
	rows, _ := s.db.Query(ctx, fmt.Sprintf("SELECT id, name FROM %s", table))
	defer rows.Close()
	var res []dbEnt
	for rows.Next() {
		var e dbEnt
		rows.Scan(&e.ID, &e.Name)
		res = append(res, e)
	}
	return res
}

func (s *EquipImportService) safeGet(row []string, idx int) string {
	if idx < 0 || idx >= len(row) { return "" }
	return strings.TrimSpace(row[idx])
}

func (s *EquipImportService) getOrCreate(ctx context.Context, table, val, col string) uint64 {
	var id uint64
	_ = s.db.QueryRow(ctx, fmt.Sprintf("SELECT id FROM %s WHERE %s = $1", table, col), val).Scan(&id)
	if id == 0 {
		_ = s.db.QueryRow(ctx, fmt.Sprintf("INSERT INTO %s (%s, created_at, updated_at) VALUES ($1, NOW(), NOW()) RETURNING id", table, col), val).Scan(&id)
	}
	return id
}
