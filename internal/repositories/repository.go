package repositories

import (
	"context"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Join struct {
	Table    string
	Alias    string
	OnLeft   string
	OnRight  string
	JoinType string
}

func (j *Join) EffectiveAlias() string {
	if j.Alias != "" {
		return j.Alias
	}
	return getBaseName(j.Table)
}

func (j *Join) SQLIdentifierForJoin() string {
	alias := j.EffectiveAlias()
	baseTable := getBaseName(j.Table)

	if alias != "" && alias != baseTable && alias != j.Table {
		return fmt.Sprintf("%s AS %s", j.Table, alias)
	}
	return j.Table
}

type Params struct {
	WithPg                bool
	Table                 string
	Alias                 string
	Columns               string
	Relations             []Join
	Filter                map[string]interface{}
	Where                 map[interface{}]interface{}
	Limit                 uint64
	Offset                uint64
	Search                string
	AllowedFilterCollumns []string
	AllowedSearchCollumns []string

	GroupRelatedFieldsByPrefix bool
}

func (p *Params) EffectiveBaseAlias() string {
	if p.Alias != "" {
		return p.Alias
	}
	return getBaseName(p.Table)
}

func contains(list []string, item string) bool {
	for _, val := range list {
		if strings.EqualFold(val, item) {
			return true
		}
	}
	return false
}

func getBaseName(identifier string) string {
	parts := strings.Split(identifier, ".")
	return parts[len(parts)-1]
}

func getColumnsForSelect(
	ctx context.Context,
	dbPool *pgxpool.Pool,
	tableSQLName string,
	prefixForColumnAlias string,
	tableIdentifierInQuery string,
	isBaseTable bool,
	groupRelatedFieldsByPrefix bool,
) (string, error) {
	actualTableNameForSchema := getBaseName(tableSQLName)
	query := `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_name = $1
		ORDER BY ordinal_position
	`
	rows, err := dbPool.Query(ctx, query, actualTableNameForSchema)
	if err != nil {
		return "", fmt.Errorf("failed to query columns for table %s: %w", actualTableNameForSchema, err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var colName string
		if err := rows.Scan(&colName); err != nil {
			return "", fmt.Errorf("scanning column name for table %s: %w", actualTableNameForSchema, err)
		}

		var selectAlias string
		if groupRelatedFieldsByPrefix {
			if isBaseTable {
				selectAlias = fmt.Sprintf("%s_%s", prefixForColumnAlias, colName)
			} else {
				selectAlias = fmt.Sprintf("%s_%s", prefixForColumnAlias, colName)
			}
		} else {
			selectAlias = fmt.Sprintf("%s_%s", prefixForColumnAlias, colName)
		}
		cols = append(cols, fmt.Sprintf("%s.%s AS %s", tableIdentifierInQuery, colName, selectAlias))
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("rows error after querying columns for %s: %w", actualTableNameForSchema, err)
	}
	return strings.Join(cols, ", "), nil
}

func applyQueryConditions(builder sq.SelectBuilder, params Params) (sq.SelectBuilder, error) {
	if len(params.Filter) > 0 {
		for key, val := range params.Filter {
			if contains(params.AllowedFilterCollumns, key) {
				builder = builder.Where(sq.Eq{key: val})
			}
		}
	}

	if len(params.Where) > 0 {
		for k, v := range params.Where {
			keyStr, ok := k.(string)
			if !ok {
				return builder, fmt.Errorf("invalid key type in Where params: key must be string, got %T", k)
			}
			builder = builder.Where(sq.Eq{keyStr: v})
		}
	}

	if params.Search != "" && len(params.AllowedSearchCollumns) > 0 {
		var conditions []sq.Sqlizer
		for _, col := range params.AllowedSearchCollumns {
			pattern := fmt.Sprintf("%%%s%%", params.Search)
			conditions = append(conditions, sq.Expr(fmt.Sprintf("%s ILIKE ?", col), pattern))
		}
		builder = builder.Where(sq.Or(conditions))
	}

	for _, join := range params.Relations {
		onClause := fmt.Sprintf("%s = %s", join.OnLeft, join.OnRight)
		joinTarget := join.SQLIdentifierForJoin()
		switch strings.ToUpper(join.JoinType) {
		case "LEFT":
			builder = builder.LeftJoin(fmt.Sprintf("%s ON %s", joinTarget, onClause))
		case "RIGHT":
			builder = builder.RightJoin(fmt.Sprintf("%s ON %s", joinTarget, onClause))
		case "INNER":
			builder = builder.Join(fmt.Sprintf("%s ON %s", joinTarget, onClause))
		default:
			builder = builder.Join(fmt.Sprintf("%s ON %s", joinTarget, onClause))
		}
	}
	return builder, nil
}

func groupRowDataSmartly(
	values []interface{},
	fieldDescriptions []pgconn.FieldDescription,
	baseTableAlias string,
	relationAliases map[string]bool,
) map[string]interface{} {
	result := make(map[string]interface{})
	relationSubMaps := make(map[string]map[string]interface{})

	for relAlias := range relationAliases {
		relationSubMaps[relAlias] = make(map[string]interface{})
	}

	for i, fd := range fieldDescriptions {
		sqlColName := string(fd.Name)
		val := values[i]

		parts := strings.SplitN(sqlColName, "_", 2)
		isPotentiallyPrefixed := len(parts) == 2

		if isPotentiallyPrefixed {
			prefixCandidate := parts[0]
			fieldNameWithoutPrefix := parts[1]

			if _, isRelation := relationAliases[prefixCandidate]; isRelation {
				relationSubMaps[prefixCandidate][fieldNameWithoutPrefix] = val
			} else if prefixCandidate == baseTableAlias {
				result[fieldNameWithoutPrefix] = val
			} else {
				result[sqlColName] = val
			}
		} else {
			result[sqlColName] = val
		}
	}

	for key, subMap := range relationSubMaps {
		if len(subMap) > 0 {
			result[key] = subMap
		}
	}
	return result
}

func FetchDataAndCount(ctx context.Context, dbPool *pgxpool.Pool, params Params) ([]map[string]interface{}, uint64, error) {
	if params.Table == "" {
		return nil, 0, fmt.Errorf("params.Table cannot be empty")
	}

	selectColumns := params.Columns
	baseTableQueryAlias := params.EffectiveBaseAlias()

	if params.Columns == "" {
		var generatedColsList []string

		baseColsStr, err := getColumnsForSelect(ctx, dbPool, params.Table, baseTableQueryAlias, baseTableQueryAlias, true, params.GroupRelatedFieldsByPrefix)
		if err != nil {
			return nil, 0, fmt.Errorf("getColumnsForSelect for base table '%s': %w", params.Table, err)
		}
		if baseColsStr != "" {
			generatedColsList = append(generatedColsList, baseColsStr)
		}

		for _, rel := range params.Relations {
			relationQueryAlias := rel.EffectiveAlias()
			relColsStr, err := getColumnsForSelect(ctx, dbPool, rel.Table, relationQueryAlias, relationQueryAlias, false, params.GroupRelatedFieldsByPrefix)
			if err != nil {
				return nil, 0, fmt.Errorf("getColumnsForSelect for relation '%s': %w", rel.Table, err)
			}
			if relColsStr != "" {
				generatedColsList = append(generatedColsList, relColsStr)
			}
		}
		selectColumns = strings.Join(generatedColsList, ", ")
		if selectColumns == "" {
			return nil, 0, fmt.Errorf("no columns could be generated for the query with auto-generation")
		}
	}

	fromTarget := params.Table
	if baseTableQueryAlias != "" && baseTableQueryAlias != getBaseName(params.Table) && baseTableQueryAlias != params.Table {
		fromTarget = fmt.Sprintf("%s AS %s", params.Table, baseTableQueryAlias)
	}

	builder := sq.Select(selectColumns).From(fromTarget).PlaceholderFormat(sq.Dollar)
	builder, err := applyQueryConditions(builder, params)
	if err != nil {
		return nil, 0, fmt.Errorf("applyQueryConditions for data: %w", err)
	}

	if params.WithPg && params.Limit > 0 {
		builder = builder.Limit(params.Limit).Offset(params.Offset)
	}

	sqlQuery, args, err := builder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("ToSql for data query ('%s'): %w", selectColumns, err)
	}

	fmt.Printf("QUERY: %s \n\n", sqlQuery)
	rows, err := dbPool.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("db.Query failed for SQL: '%s' with args %v: %w", sqlQuery, args, err)
	}
	defer rows.Close()

	var resultData []map[string]interface{}
	fieldDescriptions := rows.FieldDescriptions()

	relationAliasesForGrouping := make(map[string]bool)
	if params.GroupRelatedFieldsByPrefix {
		for _, rel := range params.Relations {
			relationAliasesForGrouping[rel.EffectiveAlias()] = true
		}
	}

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, 0, fmt.Errorf("rows.Values: %w", err)
		}

		var rowMap map[string]interface{}
		if params.GroupRelatedFieldsByPrefix {
			rowMap = groupRowDataSmartly(values, fieldDescriptions, baseTableQueryAlias, relationAliasesForGrouping)
		} else {
			rowMap = make(map[string]interface{})
			for i, fd := range fieldDescriptions {
				rowMap[string(fd.Name)] = values[i]
			}
		}
		resultData = append(resultData, rowMap)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows.Err: %w", err)
	}

	var total uint64
	if params.WithPg {
		countFromTarget := params.Table
		if params.Alias != "" && params.Alias != getBaseName(params.Table) && params.Alias != params.Table {
			countFromTarget = fmt.Sprintf("%s AS %s", params.Table, params.Alias)
		}

		countBuilderBase := sq.Select("COUNT(*)").From(countFromTarget).PlaceholderFormat(sq.Dollar)
		countParams := Params{
			Table:                 params.Table,
			Alias:                 params.Alias,
			Relations:             params.Relations,
			Filter:                params.Filter,
			Where:                 params.Where,
			Search:                params.Search,
			AllowedFilterCollumns: params.AllowedFilterCollumns,
			AllowedSearchCollumns: params.AllowedSearchCollumns,
		}
		countBuilder, errApply := applyQueryConditions(countBuilderBase, countParams)
		if errApply != nil {
			return nil, 0, fmt.Errorf("applyQueryConditions for count: %w", errApply)
		}
		countSQL, countArgs, errToSql := countBuilder.ToSql()
		if errToSql != nil {
			return nil, 0, fmt.Errorf("count ToSql: %w", errToSql)
		}
		errScan := dbPool.QueryRow(ctx, countSQL, countArgs...).Scan(&total)
		if errScan != nil {
			return nil, 0, fmt.Errorf("count query failed for SQL ('%s'): %w", countSQL, errScan)
		}
	}

	return resultData, total, nil
}
