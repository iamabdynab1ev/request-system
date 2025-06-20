package utils

import "strconv"

func Pagination(pageInput, limitInput interface{}) (int, int) {
	page, limit := 1, 10

	switch v := pageInput.(type) {
	case string:
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	case int:
		if v > 0 {
			page = v
		}
	}

	switch v := limitInput.(type) {
	case string:
		if l, err := strconv.Atoi(v); err == nil && l > 0 {
			limit = l
		}
	case int:
		if v > 0 {
			limit = v
		}
	}

	return page, limit
}