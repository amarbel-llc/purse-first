package output

// ArrayLimits controls pagination of array results.
// Zero values mean unlimited (Limit) or no offset (Offset).
type ArrayLimits struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// PaginationInfo describes the pagination applied to an array result.
type PaginationInfo struct {
	Offset  int  `json:"offset"`
	Limit   int  `json:"limit"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

// LimitedArray is the result of applying ArrayLimits to a slice.
type LimitedArray[T any] struct {
	Items      []T            `json:"items"`
	Truncated  bool           `json:"truncated"`
	TotalCount int            `json:"total_count"`
	Pagination PaginationInfo `json:"pagination"`
}

// LimitArray applies pagination limits to a slice.
// The returned Items is a subslice (no copy). Offset is clamped to the slice length.
func LimitArray[T any](items []T, limits ArrayLimits) LimitedArray[T] {
	total := len(items)

	offset := limits.Offset
	if offset < 0 {
		offset = 0
	}

	if offset > total {
		offset = total
	}

	result := items[offset:]

	limit := limits.Limit
	hasMore := false

	if limit > 0 && limit < len(result) {
		result = result[:limit]
		hasMore = true
	}

	effectiveLimit := limit
	if effectiveLimit <= 0 {
		effectiveLimit = total
	}

	truncated := len(result) != total

	return LimitedArray[T]{
		Items:      result,
		Truncated:  truncated,
		TotalCount: total,
		Pagination: PaginationInfo{
			Offset:  offset,
			Limit:   effectiveLimit,
			Total:   total,
			HasMore: hasMore,
		},
	}
}
