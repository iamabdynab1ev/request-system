package services

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"
)

var expectedColumns = map[string]string{
	"atm":      "номер банкоматов",
	"terminal": "номер терминала",
	"pos":      "номер пос-терминалов",
}

type dbEnt struct {
	ID   uint64
	Name string
}

type EquipImportService struct {
	db *pgxpool.Pool
}

func NewEquipImportService(db *pgxpool.Pool) *EquipImportService {
	return &EquipImportService{db: db}
}

func (s *EquipImportService) ImportAtmsReader(r io.Reader) error {
	return s.masterImportReader(r, "Банкомат")
}
func (s *EquipImportService) ImportPosReader(r io.Reader) error {
	return s.masterImportReader(r, "Пос-терминал")
}
func (s *EquipImportService) ImportTerminalsReader(r io.Reader) error {
	return s.masterImportReader(r, "ТЕРМИНАЛ_СМАРТ")
}

func (s *EquipImportService) masterImportReader(r io.Reader, targetType string) error {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return fmt.Errorf("ошибка открытия файла: %w", err)
	}
	defer f.Close()

	ctx := context.Background()

	// Начинаем транзакцию
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback(ctx) // откат если что-то пошло не так

	branchData := s.getRawEntities(ctx, "branches")
	officeData := s.getRawEntities(ctx, "offices")
	statusID := s.getOrCreate(ctx, "statuses", "ACTIVE", "code")

	vnutrTypeID := s.getOrCreate(ctx, "equipment_types", "Внутренний терминал", "name")
	vneshTypeID := s.getOrCreate(ctx, "equipment_types", "Внешний терминал", "name")
	cashTypeID := s.getOrCreate(ctx, "equipment_types", "Терминал Cash-in/out", "name")

	var defaultTypeID uint64
	if targetType != "ТЕРМИНАЛ_СМАРТ" {
		defaultTypeID = s.getOrCreate(ctx, "equipment_types", targetType, "name")
	}

	success, updated := 0, 0
	processedNames := []string{}
	touchedTypesMap := make(map[uint64]bool)

	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}

		fmt.Printf("🔍 Анализирую лист: %s\n", sheet)

		var bIdx, nIdx, oIdx, aIdx, kIdx = -1, -1, -1, -1, -1
		var headerFoundRow = -1

		for rIdx, row := range rows {
			rowStr := strings.ToLower(strings.Join(row, "|"))
			if strings.Contains(rowStr, "филиал") || strings.Contains(rowStr, "номер") || strings.Contains(rowStr, "№") {
				for cIdx, colName := range row {
					cLower := strings.ToLower(strings.TrimSpace(colName))
					if strings.Contains(cLower, "филиал") {
						bIdx = cIdx
					}
					if strings.Contains(cLower, "номер") || strings.Contains(cLower, "№") {
						nIdx = cIdx
					}
					if strings.Contains(cLower, "цбо") || strings.Contains(cLower, "территор") {
						oIdx = cIdx
					}
					if strings.Contains(cLower, "адрес") {
						aIdx = cIdx
					}
					if strings.Contains(cLower, "вид") || strings.Contains(cLower, "тип") {
						kIdx = cIdx
					}
				}
				if nIdx != -1 {
					headerFoundRow = rIdx
					break
				}
			}
		}

		if headerFoundRow == -1 {
			continue
		}

		for i := headerFoundRow + 1; i < len(rows); i++ {
			row := rows[i]
			if len(row) < 2 {
				continue
			}

			name := s.safeGet(row, nIdx)
			if name == "" || s.isTrash(name) {
				continue
			}

			branchName := s.safeGet(row, bIdx)
			officeName := s.safeGet(row, oIdx)
			address := s.safeGet(row, aIdx)
			vidText := strings.ToLower(s.safeGet(row, kIdx))

			var finalTypeID uint64
			if targetType == "ТЕРМИНАЛ_СМАРТ" {
				if strings.Contains(vidText, "внеш") {
					finalTypeID = vneshTypeID
				} else if strings.Contains(vidText, "внутр") {
					finalTypeID = vnutrTypeID
				} else if strings.Contains(vidText, "cash") {
					finalTypeID = cashTypeID
				} else {
					// Вид не определён — откат транзакции
					return fmt.Errorf("строка %d: [%s] вид терминала не определён: '%s' — импорт отменён", i+1, name, vidText)
				}
			} else {
				finalTypeID = defaultTypeID
			}

			touchedTypesMap[finalTypeID] = true
			processedNames = append(processedNames, name)

			if address == "" {
				if officeName != "" {
					address = officeName
				} else if branchName != "" {
					address = branchName
				} else {
					address = "-"
				}
			}

			bID := s.fuzzyFind(branchName, branchData)
			oID := s.fuzzyFind(officeName, officeData)

			// Собираем все незаполненные поля
			missingFields := []string{}
			if branchName != "" && bID == 0 {
				missingFields = append(missingFields, fmt.Sprintf("филиал='%s'", branchName))
			}
			if officeName != "" && oID == 0 {
				missingFields = append(missingFields, fmt.Sprintf("офис/цбо='%s'", officeName))
			}
			if address == "" {
				missingFields = append(missingFields, "адрес=пусто")
			}
			if vidText == "" && targetType == "ТЕРМИНАЛ_СМАРТ" {
				missingFields = append(missingFields, "вид терминала=пусто")
			}

			if len(missingFields) > 0 {
				fmt.Printf("⚠️ Стр %d: [%s] Не найдены/не заполнены: %s\n", i+1, name, strings.Join(missingFields, " | "))
			}

			var dbBID interface{} = nil
			if bID > 0 {
				dbBID = bID
			}
			var dbOID interface{} = nil
			if oID > 0 {
				dbOID = oID
			}

			query := `
    INSERT INTO equipments (name, address, branch_id, office_id, status_id, equipment_type_id, updated_at)
    VALUES ($1, $2, $3, $4, $5, $6, NOW())
    ON CONFLICT (name) 
    DO UPDATE SET 
        address = COALESCE(NULLIF(EXCLUDED.address, '-'), equipments.address),
        equipment_type_id = EXCLUDED.equipment_type_id,
        updated_at = NOW(),
        -- Обновляем филиал/офис ТОЛЬКО если в Excel нашлась связка
        -- Если не нашлась ($3 IS NULL) — не трогаем то что установлено вручную
        branch_id = CASE 
            WHEN $3 IS NOT NULL THEN $3 
            ELSE equipments.branch_id 
        END,
        office_id = CASE 
            WHEN $4 IS NOT NULL THEN $4 
            ELSE equipments.office_id 
        END
    RETURNING (xmax = 0) AS is_insert`

			var isInsert bool
			err = tx.QueryRow(ctx, query, name, address, dbBID, dbOID, statusID, finalTypeID).Scan(&isInsert)
			if err != nil {
				return fmt.Errorf("строка %d: [%s] ошибка БД: %w — импорт отменён", i+1, name, err)
			}

			if isInsert {
				success++
			} else {
				updated++
			}
		}
	}

	// Удаляем устаревшие записи
	if len(processedNames) > 0 {
		typeIDs := []uint64{}
		for tID := range touchedTypesMap {
			typeIDs = append(typeIDs, tID)
		}

		delQuery := `DELETE FROM equipments WHERE equipment_type_id = ANY($1) AND name != ALL($2)`
		cmdTag, delErr := tx.Exec(ctx, delQuery, typeIDs, processedNames)
		if delErr != nil {
			return fmt.Errorf("ошибка при удалении устаревших записей: %w — импорт отменён", delErr)
		}
		deletedCount := cmdTag.RowsAffected()
		if deletedCount > 0 {
			fmt.Printf("🧹 Очистка: удалено %d записей, отсутствующих в Excel.\n", deletedCount)
		}
	}

	// Всё хорошо — фиксируем транзакцию
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ошибка фиксации транзакции: %w", err)
	}

	fmt.Printf("\n🏁 ИТОГ: Новых: %d | Обновлено: %d\n", success, updated)
	return nil
}
func (s *EquipImportService) fuzzyFind(excelName string, dbItems []dbEnt) uint64 {
	excelName = strings.ToLower(strings.TrimSpace(excelName))
	if excelName == "" {
		return 0
	}
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
	in = strings.NewReplacer(
		"ҳ", "х",
		"ҷ", "ч",
		"қ", "к",
		"ӯ", "у",
		"ӣ", "и",
		"ғ", "г",
		"ӡ", "з",
	).Replace(strings.ToLower(in))

	replacer := strings.NewReplacer(
		"филиали", "", "филиал", "",
		"чсп бонки арванд", "", "жсп бонки арванд", "",
		"чсп", "", "жсп", "",
		"бонки", "", "арванд", "",
		"хиёбони исмоили сомони", "",
		"маркази филиали", "", "маркази", "", "марказ", "",
		"шахри", "", "шари", "",
		"дар", "", "мхб", "", "цбо", "",
		"н.", "", "ш.", "",
		"\"", "", "«", "", "»", "",
		" ", "", ".", "", "-", "",
		"район", "", "обслуживания", "",
		"мхмх", "", "г.", "",
	)
	return strings.TrimSpace(replacer.Replace(in))
}

func (s *EquipImportService) isTrash(val string) bool {
	v := strings.ToLower(strings.TrimSpace(val))
	if v == "" || strings.Contains(v, "итого") || strings.Contains(v, "всего") {
		return true
	}
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
	if idx < 0 || idx >= len(row) {
		return ""
	}
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
func (s *EquipImportService) ValidateFileType(filePath string, fileType string) error {
	expected, ok := expectedColumns[fileType]
	if !ok {
		return fmt.Errorf("неизвестный тип: %s", fileType)
	}

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return fmt.Errorf("ошибка открытия файла: %w", err)
	}
	defer f.Close()

	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		for _, row := range rows {
			for _, cell := range row {
				if strings.Contains(strings.ToLower(strings.TrimSpace(cell)), expected) {
					return nil // нашли — файл правильный
				}
			}
			// Проверяем только первые 5 строк
			break
		}
		// Проверяем первые 5 строк каждого листа
		for i, row := range rows {
			if i >= 5 {
				break
			}
			for _, cell := range row {
				if strings.Contains(strings.ToLower(strings.TrimSpace(cell)), expected) {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("файл не похож на тип '%s' — колонка '%s' не найдена. Проверьте что загружаете правильный файл", fileType, expected)
}
func (s *EquipImportService) DetectFileTypeReader(r io.Reader) (string, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return "", fmt.Errorf("ошибка открытия файла: %w", err)
	}
	defer f.Close()

	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		for i, row := range rows {
			if i >= 5 {
				break
			}
			for _, cell := range row {
				cell = strings.ToLower(strings.TrimSpace(cell))
				if strings.Contains(cell, "номер банкоматов") {
					return "atm", nil
				}
				if strings.Contains(cell, "номер терминала") {
					return "terminal", nil
				}
				if strings.Contains(cell, "номер пос-терминалов") {
					return "pos", nil
				}
			}
		}
	}

	return "", fmt.Errorf("не удалось определить тип файла — загрузите файл банкоматов, терминалов или пос-терминалов")
}
