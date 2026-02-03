package bd

import (
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"request-system/pkg/types"
)

// ApplyListParams применяет фильтры, сортировку и пагинацию.
func ApplyListParams(builder sq.SelectBuilder, filter types.Filter, allowedMap map[string]string) sq.SelectBuilder {
	// 1. Фильтрация
	for jsonField, val := range filter.Filter {
		dbCol, ok := allowedMap[jsonField]
		if !ok {
			continue
		}

		// FIX: Если пришла строка "1,2,3" -> делаем IN ("1","2","3")
		if s, ok := val.(string); ok && strings.Contains(s, ",") {
			builder = builder.Where(sq.Eq{dbCol: strings.Split(s, ",")})
		} else {
			builder = builder.Where(sq.Eq{dbCol: val})
		}
	}

	// 2. Сортировка
	if len(filter.Sort) > 0 {
		for jsonField, dir := range filter.Sort {
			dbCol, ok := allowedMap[jsonField]
			if !ok {
				continue
			}
			sqlDir := "ASC"
			if strings.ToLower(dir) == "desc" {
				sqlDir = "DESC"
			}
			builder = builder.OrderBy(fmt.Sprintf("%s %s", dbCol, sqlDir))
		}
	}

	// 3. Пагинация
	if filter.WithPagination {
		if filter.Limit > 0 {
			builder = builder.Limit(uint64(filter.Limit))
		}
		if filter.Offset >= 0 {
			builder = builder.Offset(uint64(filter.Offset))
		}
	}

	return builder
}
