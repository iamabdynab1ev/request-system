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
func (s *EquipImportService) ImportTerminals(path string) error { return s.masterImport(path, "TERMINAL_LOGIC") }
func (s *EquipImportService) ImportPos(path string) error       { return s.masterImport(path, "Пос-терминал") }

func (s *EquipImportService) masterImport(filePath string, targetType string) error {
	f, err := excelize.OpenFile(filePath)
	if err != nil { return fmt.Errorf("ошибка открытия файла: %w", err) }
	defer f.Close()

	var finalRows [][]string
	// Инициализируем индексы как -1
	var bIdx, nIdx, oIdx, aIdx = -1, -1, -1, -1 
	var headerFoundRow = -1

	fmt.Printf("\n🚀 НАЧИНАЮ ПОИСК ЗАГОЛОВКОВ В ФАЙЛЕ: %s\n", filePath)

	for _, sheet := range f.GetSheetList() {
		rows, _ := f.GetRows(sheet)
		for rIdx, row := range rows {
			rowStr := strings.ToLower(strings.Join(row, "|"))

			// Ищем строку, где есть (Филиал ИЛИ Адрес) И (Номер ИЛИ №)
			hasPlace := strings.Contains(rowStr, "филиал") || strings.Contains(rowStr, "адрес")
			hasNum := strings.Contains(rowStr, "номер") || strings.Contains(rowStr, "№")

			if hasPlace && hasNum {
				for cIdx, colName := range row {
					cLower := strings.ToLower(strings.TrimSpace(colName))
					
					if strings.Contains(cLower, "филиал") { bIdx = cIdx }
					
					// Поддержка "Номер" и "№"
					if strings.Contains(cLower, "номер") || strings.Contains(cLower, "№") { nIdx = cIdx }
					
					// ЦБО / Офис / Территория / УЧР
					if strings.Contains(cLower, "цбо") || strings.Contains(cLower, "учр") || 
					   strings.Contains(cLower, "территория") || strings.Contains(cLower, "офис") { oIdx = cIdx }
					
					if strings.Contains(cLower, "адрес") || strings.Contains(cLower, "место") { aIdx = cIdx }
				}

				if nIdx != -1 && (bIdx != -1 || aIdx != -1) {
					finalRows = rows
					headerFoundRow = rIdx
					fmt.Printf("✅ Заголовки найдены на строке %d (Лист: %s)\n", rIdx+1, sheet)
					break
				}
			}
		}
		if headerFoundRow != -1 { break }
	}

	if headerFoundRow == -1 {
		return fmt.Errorf("НЕ НАЙДЕНА ШАПКА ТАБЛИЦЫ. Проверьте, что в файле есть строки с '№/Номер' и 'Филиал/Адрес'")
	}

	ctx := context.Background()
	branchData := s.getRawEntities(ctx, "branches")
	officeData := s.getRawEntities(ctx, "offices")
	
	statusID := s.getOrCreate(ctx, "statuses", "ACTIVE", "code")
	typeID := s.getOrCreate(ctx, "equipment_types", targetType, "name")

	success, errors, updated := 0, 0, 0
	
	// --- ЦИКЛ ИМПОРТА ---
	for i := headerFoundRow + 1; i < len(finalRows); i++ {
		row := finalRows[i]
		if len(row) < 2 { continue }

		lineNum := i + 1

		name := s.safeGet(row, nIdx)
		
		// Если это мусор или пустота - пропускаем
		if name == "" { continue }
		if s.isTrash(name) { 
			// fmt.Printf("ℹ️  Стр %d: Пропущено (мусор/нумерация): '%s'\n", lineNum, name)
			continue 
		}

		branchName := s.safeGet(row, bIdx)
		officeName := s.safeGet(row, oIdx)
		address    := s.safeGet(row, aIdx)

		// Если адрес пуст, пробуем заполнить его данными офиса/филиала
		// Это важно, чтобы SQL не падал, если address NOT NULL (в вашей миграции он остался NOT NULL)
		if address == "" {
			if officeName != "" { address = officeName } else if branchName != "" { address = branchName } else { address = "-" }
		}

		// Ищем в БД
		bID := s.fuzzyFind(branchName, branchData)
		oID := s.fuzzyFind(officeName, officeData)

		// Логируем только если название было, но мы его не нашли
		if bID == 0 && branchName != "" {
			fmt.Printf("⚠️  Стр %d [%s]: Филиал '%s' не найден в базе (привязка будет пропущена)\n", lineNum, name, branchName)
		}
		
		// Подготовка значений (nil превращается в NULL)
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
                branch_id = COALESCE(EXCLUDED.branch_id, equipments.branch_id), 
                office_id = COALESCE(EXCLUDED.office_id, equipments.office_id), 
                updated_at = NOW()
            RETURNING (xmax = 0) AS is_insert`
		
		var isInsert bool
		err = s.db.QueryRow(ctx, query, name, address, dbBID, dbOID, statusID, typeID).Scan(&isInsert)

		if err != nil {
			fmt.Printf("❌ Стр %d [%s]: ОШИБКА SQL: %v\n", lineNum, name, err)
			errors++
		} else {
			if isInsert { success++ } else { updated++ }
		}
	}

	fmt.Printf("---------------------------------------------------------\n")
	fmt.Printf("🏁 РЕЗУЛЬТАТ ИМПОРТА %s (%s):\n", targetType, filePath)
	fmt.Printf("   ✅ Новых записей:    %d\n", success)
	fmt.Printf("   🔄 Обновлено записей: %d\n", updated)
	fmt.Printf("   ❌ Ошибок:            %d\n", errors)
	fmt.Printf("---------------------------------------------------------\n")
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
	
	if v == "" { return true }
	
	if strings.Contains(v, "итого") || strings.Contains(v, "всего") { return true }
	
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
