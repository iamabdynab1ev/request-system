package utils

import (
	"net/url"
	"strconv"
	"strings"
)

type QueryParams struct {
	Filters   map[string]string
	Search    string
	SortBy    string
	SortOrder string
	Limit     uint64
	Offset    uint64
	Page      uint64
}

func ParseQuery(query url.Values) QueryParams {
	params := QueryParams{
		Filters:   make(map[string]string),
		Limit:     10,
		Offset:    0,
		Page:      1,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	for key, values := range query {
		if strings.HasPrefix(key, "filter[") && strings.HasSuffix(key, "]") && len(values) > 0 {
			filterKey := key[7 : len(key)-1]
			params.Filters[filterKey] = values[0]
		}
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		if l, err := strconv.ParseUint(limitStr, 10, 64); err == nil && l > 0 {
			params.Limit = l
		}
	}
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if o, err := strconv.ParseUint(offsetStr, 10, 64); err == nil {
			params.Offset = o
			if params.Limit > 0 {
				params.Page = (o / params.Limit) + 1
			}
		}
	}
	if pageStr := query.Get("page"); pageStr != "" && params.Offset == 0 { // page имеет приоритет только если offset не задан
		if p, err := strconv.ParseUint(pageStr, 10, 64); err == nil && p > 0 {
			params.Page = p
			params.Offset = (p - 1) * params.Limit
		}
	}

	if search := query.Get("search"); search != "" {
		params.Search = search
	}

	if sort := query.Get("sort"); sort != "" {
		if strings.HasPrefix(sort, "-") {
			params.SortOrder = "desc"
			params.SortBy = sort[1:]
		} else {
			params.SortOrder = "asc"
			params.SortBy = sort
		}
	}
	return params
}
