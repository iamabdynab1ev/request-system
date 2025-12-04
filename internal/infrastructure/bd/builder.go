package db

import (
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"

	"request-system/pkg/types"
)

func ApplyListParams(builder sq.SelectBuilder, filter types.Filter, allowedMap map[string]string) sq.SelectBuilder {
	for jsonField, val := range filter.Filter {
		dbCol, ok := allowedMap[jsonField]
		if !ok {
			continue
		}

		if s, ok := val.(string); ok && strings.Contains(s, ",") {
			builder = builder.Where(sq.Eq{dbCol: strings.Split(s, ",")})
		} else {
			builder = builder.Where(sq.Eq{dbCol: val})
		}
	}

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
