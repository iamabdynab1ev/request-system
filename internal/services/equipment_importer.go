package services

import (
	"context"
	"fmt"
	"strconv"
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
func (s *EquipImportService) ImportTerminals(path string) error { return s.masterImport(path, "TERMINAL_LOGIC") }
func (s *EquipImportService) ImportPos(path string) error       { return s.masterImport(path, "Пос-терминал") }

func (s *EquipImportService) masterImport(filePath string, targetType string) error {
	f, err := excelize.OpenFile(filePath)
	if err != nil { return fmt.Errorf("ошибка открытия файла: %w", err) }
	defer f.Close()

	var finalRows [][]string
	var bIdx, nIdx, oIdx, aIdx, kIdx = -1, -1, -1, -1, -1
	var headerFoundRow = -1

	// Поиск данных в листах
	for _, sheet := range f.GetSheetList() {
		rows, _ := f.GetRows(sheet)
		for rIdx, row := range rows {
			rowStr := strings.ToLower(strings.Join(row, "|"))
			if strings.Contains(rowStr, "филиал") && (strings.Contains(rowStr, "номер") || strings.Contains(rowStr, "банкомат")) {
				finalRows = rows
				headerFoundRow = rIdx
				for cIdx, colName := range row {
					cLower := strings.ToLower(strings.TrimSpace(colName))
					if strings.Contains(cLower, "филиал") { bIdx = cIdx }
					if strings.Contains(cLower, "номер") { nIdx = cIdx }
					if strings.Contains(cLower, "цбо") || strings.Contains(cLower, "офис") || strings.Contains(cLower, "территор") { oIdx = cIdx }
					if strings.Contains(cLower, "адрес") { aIdx = cIdx }
					if strings.Contains(cLower, "вид") { kIdx = cIdx }
				}
				break
			}
		}
		if headerFoundRow != -1 { break }
	}

	if headerFoundRow == -1 { return fmt.Errorf("в файле не найдена таблица (заголовки Филиал/Номер)") }

	ctx := context.Background()
	branchData := s.getRawEntities(ctx, "branches")
	officeData := s.getRawEntities(ctx, "offices")
	statusID := s.getOrCreate(ctx, "statuses", "ACTIVE", "code")

	notFoundBranches := make(map[string]bool)
	notFoundOffices := make(map[string]bool)
	countSuccess := 0

	for i := headerFoundRow + 1; i < len(finalRows); i++ {
		row := finalRows[i]
		rawName   := s.safeGet(row, nIdx)
		rawBranch := s.safeGet(row, bIdx)
		rawOffice := s.safeGet(row, oIdx)
		address   := s.safeGet(row, aIdx)

		if rawName == "" || s.isTrash(rawBranch) { continue }

		bID := s.fuzzyFind(rawBranch, branchData)
		oID := s.fuzzyFind(rawOffice, officeData)

		if bID == 0 && rawBranch != "" { notFoundBranches[rawBranch] = true }
		if oID == 0 && rawOffice != "" { notFoundOffices[rawOffice] = true }

		if bID == 0 || oID == 0 { continue }

		realType := targetType
		if kIdx != -1 && targetType == "TERMINAL_LOGIC" {
			if strings.Contains(strings.ToLower(s.safeGet(row, kIdx)), "внеш") { realType = "Внешний терминал" } else { realType = "Внутренний терминал" }
		}
		typeID := s.getOrCreate(ctx, "equipment_types", realType, "name")

		query := `INSERT INTO equipments (name, address, branch_id, office_id, status_id, equipment_type_id, updated_at)
			      VALUES ($1, $2, $3, $4, $5, $6, NOW())
			      ON CONFLICT (name) DO UPDATE SET address = EXCLUDED.address, branch_id = EXCLUDED.branch_id, office_id = EXCLUDED.office_id, updated_at = NOW()`
		
		_, err = s.db.Exec(ctx, query, rawName, address, bID, oID, statusID, typeID)
		if err == nil { countSuccess++ }
	}

	fmt.Println("\n=========================================================")
	fmt.Printf("🏁 ИТОГ ИМПОРТА ДЛЯ: %s\n", filePath)
	fmt.Printf("✅ Успешно загружено/обновлено: %d ед.\n", countSuccess)
	if len(notFoundBranches) > 0 {
		fmt.Println("\n❌ НЕ НАЙДЕНЫ ФИЛИАЛЫ (исправьте в Excel):")
		for name := range notFoundBranches { fmt.Printf("   - %s\n", name) }
	}
	if len(notFoundOffices) > 0 {
		fmt.Println("\n❌ НЕ НАЙДЕНЫ ОФИСЫ (исправьте в Excel):")
		for name := range notFoundOffices { fmt.Printf("   - %s\n", name) }
	}
	fmt.Println("=========================================================\n")

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
	// Исправленный Replacer (все аргументы теперь в парах: старое, новое)
	replacer := strings.NewReplacer(
		"филиал", "", 
		"цбо", "", 
		"мхмх", "", 
		"г.", "", 
		"\"", "", 
		"«", "", 
		"»", "", 
		" ", "", 
		".", "", 
		"-", "", 
		"район", "", 
		"обслуживания", "",
	)
	return strings.TrimSpace(replacer.Replace(strings.ToLower(in)))
}

func (s *EquipImportService) isTrash(val string) bool {
	v := strings.ToLower(strings.TrimSpace(val))
	if v == "" || strings.Contains(v, "итого") || strings.Contains(v, "всего") { return true }
	if _, err := strconv.ParseFloat(v, 64); err == nil { return true }
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
		_ = s.db.QueryRow(ctx, fmt.Sprintf("INSERT INTO %s (%s) VALUES ($1) RETURNING id", table, col), val).Scan(&id)
	}
	return id
}
