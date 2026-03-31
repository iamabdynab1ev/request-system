package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
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
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewEquipImportService(db *pgxpool.Pool, logger *zap.Logger) *EquipImportService {
	return &EquipImportService{db: db, logger: logger.Named("equipment_import")}
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
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			s.logger.Warn("Не удалось откатить транзакцию импорта оборудования", zap.Error(rollbackErr))
		}
	}()

	branchData, err := s.getRawEntitiesTx(ctx, tx, "branches")
	if err != nil {
		return fmt.Errorf("ошибка загрузки филиалов: %w", err)
	}
	officeData, err := s.getRawEntitiesTx(ctx, tx, "offices")
	if err != nil {
		return fmt.Errorf("ошибка загрузки офисов: %w", err)
	}
	statusID, err := s.getOrCreateTx(ctx, tx, "statuses", "ACTIVE", "code")
	if err != nil {
		return fmt.Errorf("ошибка подготовки статуса оборудования: %w", err)
	}

	vnutrTypeID, err := s.getOrCreateTx(ctx, tx, "equipment_types", "Внутренний терминал", "name")
	if err != nil {
		return fmt.Errorf("ошибка подготовки типа оборудования 'Внутренний терминал': %w", err)
	}
	vneshTypeID, err := s.getOrCreateTx(ctx, tx, "equipment_types", "Внешний терминал", "name")
	if err != nil {
		return fmt.Errorf("ошибка подготовки типа оборудования 'Внешний терминал': %w", err)
	}
	cashTypeID, err := s.getOrCreateTx(ctx, tx, "equipment_types", "Терминал Cash-in/out", "name")
	if err != nil {
		return fmt.Errorf("ошибка подготовки типа оборудования 'Терминал Cash-in/out': %w", err)
	}

	var defaultTypeID uint64
	if targetType != "ТЕРМИНАЛ_СМАРТ" {
		defaultTypeID, err = s.getOrCreateTx(ctx, tx, "equipment_types", targetType, "name")
		if err != nil {
			return fmt.Errorf("ошибка подготовки типа оборудования '%s': %w", targetType, err)
		}
	}

	success, updated := 0, 0
	processedNames := []string{}
	touchedTypesMap := make(map[uint64]bool)

	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			s.logger.Warn("Не удалось прочитать лист Excel", zap.String("sheet", sheet), zap.Error(err))
			continue
		}

		s.logger.Info("Анализ листа импорта оборудования", zap.String("sheet", sheet), zap.String("target_type", targetType))

		var bIdx, nIdx, oIdx, aIdx, kIdx = -1, -1, -1, -1, -1
		headerFoundRow := -1

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
				switch {
				case strings.Contains(vidText, "внеш"):
					finalTypeID = vneshTypeID
				case strings.Contains(vidText, "внутр"):
					finalTypeID = vnutrTypeID
				case strings.Contains(vidText, "cash"):
					finalTypeID = cashTypeID
				default:
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
				s.logger.Warn(
					"Не найдены связанные данные при импорте оборудования",
					zap.Int("row", i+1),
					zap.String("name", name),
					zap.String("details", strings.Join(missingFields, " | ")),
				)
			}

			var dbBID interface{}
			if bID > 0 {
				dbBID = bID
			}
			var dbOID interface{}
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
					branch_id = CASE WHEN $3 IS NOT NULL THEN $3 ELSE equipments.branch_id END,
					office_id = CASE WHEN $4 IS NOT NULL THEN $4 ELSE equipments.office_id END
				RETURNING (xmax = 0) AS is_insert`

			var isInsert bool
			if err := tx.QueryRow(ctx, query, name, address, dbBID, dbOID, statusID, finalTypeID).Scan(&isInsert); err != nil {
				return fmt.Errorf("строка %d: [%s] ошибка БД: %w — импорт отменён", i+1, name, err)
			}

			if isInsert {
				success++
			} else {
				updated++
			}
		}
	}

	if len(processedNames) > 0 && len(touchedTypesMap) > 0 {
		typeIDs := make([]uint64, 0, len(touchedTypesMap))
		for tID := range touchedTypesMap {
			typeIDs = append(typeIDs, tID)
		}

		delQuery := `DELETE FROM equipments WHERE equipment_type_id = ANY($1) AND name != ALL($2)`
		cmdTag, delErr := tx.Exec(ctx, delQuery, typeIDs, processedNames)
		if delErr != nil {
			return fmt.Errorf("ошибка при удалении устаревших записей: %w — импорт отменён", delErr)
		}
		if deletedCount := cmdTag.RowsAffected(); deletedCount > 0 {
			s.logger.Info("Удалены устаревшие записи оборудования", zap.Int64("deleted", deletedCount), zap.String("target_type", targetType))
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ошибка фиксации транзакции: %w", err)
	}

	s.logger.Info("Импорт оборудования завершен", zap.String("target_type", targetType), zap.Int("created", success), zap.Int("updated", updated))
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
		"Ті", "х",
		"Т·", "ч",
		"Т›", "к",
		"УЇ", "у",
		"УЈ", "и",
		"Т“", "г",
		"УЎ", "з",
	).Replace(strings.ToLower(in))

	replacer := strings.NewReplacer(
		"филиали", "", "филиал", "",
		"чсп банки арванд", "", "жсп банки арванд", "",
		"чсп", "", "жсп", "",
		"банки", "", "арванд", "",
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
	return v == "" || strings.Contains(v, "итого") || strings.Contains(v, "всего")
}

func (s *EquipImportService) getRawEntitiesTx(ctx context.Context, tx pgx.Tx, table string) ([]dbEnt, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf("SELECT id, name FROM %s", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []dbEnt
	for rows.Next() {
		var e dbEnt
		if err := rows.Scan(&e.ID, &e.Name); err != nil {
			return nil, err
		}
		res = append(res, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *EquipImportService) safeGet(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func (s *EquipImportService) getOrCreateTx(ctx context.Context, tx pgx.Tx, table, val, col string) (uint64, error) {
	var id uint64
	selectQuery := fmt.Sprintf("SELECT id FROM %s WHERE %s = $1", table, col)
	err := tx.QueryRow(ctx, selectQuery, val).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}

	insertQuery := fmt.Sprintf("INSERT INTO %s (%s, created_at, updated_at) VALUES ($1, NOW(), NOW()) RETURNING id", table, col)
	if err := tx.QueryRow(ctx, insertQuery, val).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
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
					return nil
				}
			}
			break
		}
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
