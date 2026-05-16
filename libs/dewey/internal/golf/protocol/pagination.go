package protocol

// PaginationParams contains cursor-based pagination parameters.
type PaginationParams struct {
	// Cursor is an opaque pagination token from a previous response.
	Cursor string `json:"cursor,omitempty"`
}

// PaginatedResult provides the next cursor for pagination.
type PaginatedResult struct {
	// NextCursor is the opaque token for the next page.
	NextCursor string `json:"nextCursor,omitempty"`
}
