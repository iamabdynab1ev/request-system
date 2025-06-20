package utils

import (
	"net/url"
	"strconv"
)

const (
	DefaultLimit = 10
	MaxLimit     = 100
)

func ParsePaginationParams(values url.Values) (limit uint64, offset uint64, page uint64) {
	// Значения по умолчанию
	limit = DefaultLimit
	page = 1

	// Парсим limit
	if limitStr := values.Get("limit"); limitStr != "" {
		if l, err := strconv.ParseUint(limitStr, 10, 64); err == nil && l > 0 {
			if l > MaxLimit {
				limit = MaxLimit
			} else {
				limit = l
			}
		}
	}

	// Парсим page или offset
	if pageStr := values.Get("page"); pageStr != "" {
		if p, err := strconv.ParseUint(pageStr, 10, 64); err == nil && p > 0 {
			page = p
		}
	}

	// offset считается на основе страницы
	offset = (page - 1) * limit

	return
}
