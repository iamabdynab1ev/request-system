package repositories

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

// pgxQuerier интерфейс для совместимости с *pgx.Conn, *pgx.Pool, pgx.Tx
type pgxQuerier interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
}

// --- Именованные ошибки для более детальной обработки ---
var (
	ErrValidation          = errors.New("validation error")
	ErrDBInteraction       = errors.New("database interaction error")
	ErrInputParameter      = errors.New("invalid input parameter")
	ErrInternalLogic       = errors.New("internal logic error") // Для ошибок, которые не должны возникать при корректной логике
	ErrFeatureNotSupported = errors.New("feature not supported or not recommended due to security concerns")
)

// --- ENUM-подобные типы для ясности ---

type JoinType string

const (
	JoinInner JoinType = "INNER JOIN"
	JoinLeft  JoinType = "LEFT JOIN"
	JoinRight JoinType = "RIGHT JOIN"
	JoinFull  JoinType = "FULL OUTER JOIN"
)

// --- Структуры для параметров запроса ---

type QueryParams struct {
	FromTable     string
	FromAlias     string
	SelectColumns []string
	Joins         []JoinData
	WhereClause   string        // Безопасность структуры - ответственность вызывающей стороны. Значения через WhereArgs.
	WhereArgs     []interface{} // Если WhereClause содержит плейсхолдеры $N, здесь должны быть соответствующие значения.
	OrderBy       string
	Limit         int
	Offset        int
}

type JoinData struct {
	Type  JoinType
	Table string
	Alias string
	// OnClause: Важно! Если этот OnClause должен содержать динамические *значения*,
	// то эти значения должны передаваться через основной QueryParams.WhereArgs,
	// а сама строка OnClause должна содержать соответствующие плейсхолдеры ($N),
	// которые корректно указывают на позиции аргументов в QueryParams.WhereArgs.
	// validateRawSQLClause защищает от опасных *ключевых слов*, но не от внедрения
	// через неверно сформированные значения, если они не параметризованы.
	OnClause string
}

type QueryResult struct {
	Data  []map[string]interface{}
	Total int64
}

// --- Валидаторы и мэппинги ---

// ValidColumns определяет разрешенные таблицы и их столбцы.
// Ключ - имя таблицы, значение - слайс разрешенных столбцов.
var ValidColumns = map[string][]string{
	"order_delegations": {"id", "delegation_user_id", "delegated_user_id", "status_id", "order_id", "created_at", "updated_at"},
	"orders":            {"id", "name", "customer_id", "equipment_id", "order_date", "status", "description"}, // Добавлены для примера из main
	"customers":         {"id", "name", "region", "company_notes"},                                          // Добавлены
	"equipment":         {"id", "serial_number", "type_id", "description"},                                  // Добавлены
	"equipment_types":   {"id", "type_name"},                                                                 // Добавлены
	"statuses":          {"id", "name"},
}

// Регулярное выражение для валидации псевдонимов (простой SQL идентификатор).
var validAliasRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func isFieldInWhitelist(actualTableName string, columnName string, params QueryParams) bool {
	if columnName == "*" {
		_, tableWhitelisted := ValidColumns[actualTableName]
		return tableWhitelisted
	}
	validFields, ok := ValidColumns[actualTableName]
	if !ok {
		return false
	}
	for _, f := range validFields {
		if f == columnName {
			return true
		}
	}
	return false
}

func validateSelectColumn(colSelectExpression string, params QueryParams) error {
	partsAS := strings.SplitN(strings.TrimSpace(colSelectExpression), " AS ", 2)
	colExpr := partsAS[0]

	if strings.Contains(colExpr, "(") && strings.Contains(colExpr, ")") {
		if err := validateRawSQLClause(colExpr, "SELECT column expression (function)"); err != nil {
			return fmt.Errorf("%w: проверка SELECT колонки (функции) '%s' провалена: %v", ErrValidation, colSelectExpression, err)
		}
		return nil
	}

	partsDot := strings.Split(colExpr, ".")
	var aliasOrTableName, columnName string

	if len(partsDot) == 2 {
		aliasOrTableName = strings.TrimSpace(partsDot[0])
		columnName = strings.TrimSpace(partsDot[1])
	} else if len(partsDot) == 1 {
		if params.FromAlias != "" {
			aliasOrTableName = params.FromAlias
		} else {
			aliasOrTableName = params.FromTable
		}
		columnName = strings.TrimSpace(partsDot[0])
	} else {
		return fmt.Errorf("%w: недопустимый формат для SELECT колонки: '%s'", ErrValidation, colSelectExpression)
	}

	actualTableName := ""
	if aliasOrTableName == params.FromAlias || (params.FromAlias == "" && aliasOrTableName == params.FromTable) {
		actualTableName = params.FromTable
	} else {
		foundInJoins := false
		for _, join := range params.Joins {
			if aliasOrTableName == join.Alias {
				actualTableName = join.Table
				foundInJoins = true
				break
			}
		}
		if !foundInJoins {
			if _, tableExistsInWhitelist := ValidColumns[aliasOrTableName]; tableExistsInWhitelist {
				actualTableName = aliasOrTableName
			} else {
				return fmt.Errorf("%w: неизвестный алиас или имя таблицы '%s' в SELECT колонке: '%s'", ErrValidation, aliasOrTableName, colSelectExpression)
			}
		}
	}

	if actualTableName == "" {
		return fmt.Errorf("%w: не удалось определить имя таблицы для алиаса '%s' в SELECT колонке: '%s'", ErrInternalLogic, aliasOrTableName, colSelectExpression)
	}

	if !isFieldInWhitelist(actualTableName, columnName, params) {
		return fmt.Errorf("%w: недопустимое поле '%s' для таблицы '%s' (алиас '%s') в SELECT: '%s'",
			ErrValidation, columnName, actualTableName, aliasOrTableName, colSelectExpression)
	}
	return nil
}

func validateOrderBy(orderByClause string, params QueryParams) error {
	if orderByClause == "" {
		return nil
	}
	orderSegments := strings.Split(orderByClause, ",")
	for _, segment := range orderSegments {
		trimmedSegment := strings.TrimSpace(segment)
		if trimmedSegment == "" {
			continue
		}
		parts := strings.Fields(trimmedSegment)
		if len(parts) < 1 {
			return fmt.Errorf("%w: пустой сегмент в ORDER BY: '%s'", ErrValidation, segment)
		}

		columnToOrder := parts[0]
		if err := validateSelectColumn(columnToOrder, params); err != nil {
			return fmt.Errorf("%w: недопустимая колонка '%s' в ORDER BY: %v", ErrValidation, columnToOrder, err)
		}

		if len(parts) > 1 {
			direction := strings.ToUpper(parts[1])
			validDirection := false
			if direction == "ASC" || direction == "DESC" {
				validDirection = true
			}
			
			// Проверка NULLS FIRST/LAST после ASC/DESC
			if validDirection && len(parts) > 2 {
                 nullsOpt := strings.ToUpper(strings.Join(parts[2:], " "))
                 if nullsOpt != "NULLS FIRST" && nullsOpt != "NULLS LAST" {
                     return fmt.Errorf("%w: недопустимая опция NULLS '%s' после направления сортировки в ORDER BY для сегмента: '%s'", ErrValidation, nullsOpt, segment)
                 }
            } else if direction == "NULLS" { // Проверка column_name NULLS FIRST/LAST
				if len(parts) != 3 || (strings.ToUpper(parts[2]) != "FIRST" && strings.ToUpper(parts[2]) != "LAST") {
					return fmt.Errorf("%w: недопустимая опция NULLS '%s' в ORDER BY для сегмента: '%s'", ErrValidation, strings.Join(parts[1:], " "), segment)
				}
			} else if !validDirection && direction != "NULLS" { // Если первое слово после колонки - невалидное направление и не NULLS
				 return fmt.Errorf("%w: недопустимое направление или опция '%s' в ORDER BY для сегмента: '%s'", ErrValidation, direction, segment)
			}
		}
	}
	return nil
}

// forbiddenSQLKeywords и опасные паттерны
var forbiddenSQLKeywords = regexp.MustCompile(
	`(?i)\b(UNION|INSERT|UPDATE|DELETE|ALTER|DROP|CREATE|TRUNCATE|EXECUTE|PREPARE|DEALLOCATE|GRANT|REVOKE|--|;|/\*|\*/)\b` +
		`|(pg_sleep|waitfor delay|benchmark\(|information_schema|pg_catalog\.)`,
)

func validateRawSQLClause(clause string, clauseName string) error {
	if clause == "" {
		return nil
	}
	if forbiddenSQLKeywords.MatchString(clause) {
		matches := forbiddenSQLKeywords.FindAllString(clause, -1)
		return fmt.Errorf("%w: потенциально опасное выражение (найдено: %v) в '%s': '%s'", ErrValidation, matches, clauseName, clause)
	}
	if strings.Count(clause, "'")%2 != 0 {
		return fmt.Errorf("%w: несбалансированные одинарные кавычки в '%s': '%s'", ErrValidation, clauseName, clause)
	}
	if strings.Count(clause, "`")%2 != 0 || strings.Count(clause, `"`)%2 != 0 {
		return fmt.Errorf("%w: несбалансированные обратные/двойные кавычки в '%s': '%s'", ErrValidation, clauseName, clause)
	}
	return nil
}

// --- Основная функция ---
func FetchDataAndCount(ctx context.Context, db pgxQuerier, params QueryParams) (QueryResult, error) {
	var result QueryResult
	if db == nil {
		return result, fmt.Errorf("%w: database connection (pgxQuerier) is nil", ErrInputParameter)
	}

	// 1. Валидация FromTable и FromAlias
	if params.FromTable == "" {
		return result, fmt.Errorf("%w: FromTable is required", ErrInputParameter)
	}
	if _, tableWhitelisted := ValidColumns[params.FromTable]; !tableWhitelisted {
		return result, fmt.Errorf("%w: недопустимая или неизвестная FromTable: '%s'", ErrValidation, params.FromTable)
	}

	fromAliasResolved := params.FromAlias
	if fromAliasResolved == "" {
		fromAliasResolved = params.FromTable
	} else {
		if !validAliasRegex.MatchString(fromAliasResolved) {
			return result, fmt.Errorf("%w: недопустимый формат для FromAlias: '%s'", ErrValidation, fromAliasResolved)
		}
	}

	// 2. Валидация SelectColumns
	if len(params.SelectColumns) == 0 {
		return result, fmt.Errorf("%w: SelectColumns не может быть пустым", ErrInputParameter)
	}
	for _, col := range params.SelectColumns {
		if err := validateSelectColumn(col, params); err != nil { // err уже обернут в ErrValidation или ErrInternalLogic
			return result, fmt.Errorf("ошибка валидации SELECT колонки: %w", err)
		}
	}
	selectSQL := strings.Join(params.SelectColumns, ", ")

	// 3. Валидация и сборка JOINs
	var joinSQLs []string
	for i, join := range params.Joins {
		if _, tableWhitelisted := ValidColumns[join.Table]; !tableWhitelisted {
			return result, fmt.Errorf("%w: недопустимая таблица '%s' в JOIN #%d", ErrValidation, join.Table, i+1)
		}
		if join.Alias != "" && !validAliasRegex.MatchString(join.Alias) {
			return result, fmt.Errorf("%w: недопустимый формат для JOIN alias: '%s' (JOIN #%d)", ErrValidation, join.Alias, i+1)
		}
		if err := validateRawSQLClause(join.OnClause, fmt.Sprintf("JOIN #%d OnClause", i+1)); err != nil {
			return result, err // err уже обернут в ErrValidation
		}

		joinTablePart := join.Table
		if join.Alias != "" {
			joinTablePart = fmt.Sprintf("%s AS %s", join.Table, join.Alias)
		}
		joinSQLs = append(joinSQLs, fmt.Sprintf("%s %s ON %s", join.Type, joinTablePart, join.OnClause))
	}
	joinsCombinedSQL := strings.Join(joinSQLs, "\n")

	// 4. Валидация WhereClause
	if err := validateRawSQLClause(params.WhereClause, "WHERE"); err != nil {
		return result, err // err уже обернут в ErrValidation
	}
	whereSQL := ""
	if strings.TrimSpace(params.WhereClause) != "" {
		whereSQL = "WHERE " + params.WhereClause
	}

	// 5. Валидация OrderBy
	if err := validateOrderBy(params.OrderBy, params); err != nil { // err уже обернут в ErrValidation
		return result, fmt.Errorf("ошибка валидации ORDER BY: %w", err)
	}
	orderBySQL := ""
	if params.OrderBy != "" {
		orderBySQL = "ORDER BY " + params.OrderBy
	}

	// 6. PAGINATION
	var paginationSQLBuilder strings.Builder
	if params.Offset >= 0 { // 0 - валидное смещение
		fmt.Fprintf(&paginationSQLBuilder, "OFFSET %d", params.Offset)
	} else if params.Offset < -1 { // -1 может использоваться как "не применять смещение", но здесь делаем строже
		return result, fmt.Errorf("%w: Offset не может быть отрицательным (кроме -1 для отсутствия): %d", ErrInputParameter, params.Offset)
	}
	if params.Limit > 0 {
		if paginationSQLBuilder.Len() > 0 && params.Offset >=0 { // Добавляем пробел, если есть OFFSET
			paginationSQLBuilder.WriteString(" ")
		}
		fmt.Fprintf(&paginationSQLBuilder, "LIMIT %d", params.Limit)
	} else if params.Limit < -1 { // -1 может использоваться как "не применять лимит"
		return result, fmt.Errorf("%w: Limit не может быть отрицательным (кроме -1 для отсутствия): %d", ErrInputParameter, params.Limit)
	}
	paginationSQL := paginationSQLBuilder.String()

	// 7. Сборка итогового запроса
	var queryBuilder strings.Builder
	fmt.Fprintf(&queryBuilder, "SELECT %s, COUNT(*) OVER() AS total_count\n", selectSQL)

	fromTableSQLFormatted := params.FromTable
	if params.FromAlias != "" && params.FromAlias != params.FromTable {
		fromTableSQLFormatted = fmt.Sprintf("%s AS %s", params.FromTable, params.FromAlias)
	}
	fmt.Fprintf(&queryBuilder, "FROM %s", fromTableSQLFormatted)

	if joinsCombinedSQL != "" {
		fmt.Fprintf(&queryBuilder, "\n%s", joinsCombinedSQL)
	}
	if whereSQL != "" {
		fmt.Fprintf(&queryBuilder, "\n%s", whereSQL)
	}
	if orderBySQL != "" {
		fmt.Fprintf(&queryBuilder, "\n%s", orderBySQL)
	}
	if paginationSQL != "" {
		fmt.Fprintf(&queryBuilder, "\n%s", paginationSQL)
	}

	finalQueryString := queryBuilder.String()

	// 8. Выполнение запроса
	rows, err := db.Query(ctx, finalQueryString, params.WhereArgs...)
	if err != nil {
		// Включаем SQL в ошибку только если это не ErrValidation (чтобы не раскрывать потенциально чувствительные части WHERE)
		// или если включен специальный режим отладки. Для простоты пока так:
		return result, fmt.Errorf("%w: ошибка выполнения запроса '%s': %v. SQL: %s. Args: %v", ErrDBInteraction, finalQueryString, err, finalQueryString, params.WhereArgs)
	}
	defer rows.Close()

	rawData, errCollect := pgx.CollectRows(rows, pgx.RowToMap)
	if errCollect != nil {
		// Здесь rows.Err() тоже может быть полезен
		return result, fmt.Errorf("%w: ошибка сбора строк: %v (rows.Err(): %v)", ErrDBInteraction, errCollect, rows.Err())
	}
	// rows.Err() должен проверяться *после* rows.Close() или после полного чтения из rows (CollectRows это делает)
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("%w: ошибка итерации по строкам после сбора: %v", ErrDBInteraction, err)
	}
	
	if len(rawData) > 0 {
		totalVal, ok := rawData[0]["total_count"]
		if !ok {
			return result, fmt.Errorf("%w: колонка total_count не найдена в результате запроса (это внутренняя ошибка)", ErrInternalLogic)
		}
		total, castOk := totalVal.(int64)
		if !castOk {
			return result, fmt.Errorf("%w: неожиданный тип для total_count: %T (значение: %v), ожидался int64 (это внутренняя ошибка)", ErrInternalLogic, totalVal, totalVal)
		}
		result.Total = total
	} else {
		result.Total = 0 // Если нет данных, то и общее количество 0
	}

	result.Data = make([]map[string]interface{}, len(rawData))
	for i, rowMap := range rawData {
		cleanMap := make(map[string]interface{}, len(rowMap)-1) // -1 для total_count
		for k, v := range rowMap {
			if k != "total_count" {
				cleanMap[k] = v
			}
		}
		result.Data[i] = cleanMap
	}

	return result, nil
}