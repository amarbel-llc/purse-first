package protocol

// Root represents a root directory or file that the server can operate on.
type Root struct {
	// URI is the root's location (typically file:// scheme).
	URI string `json:"uri"`

	// Name is an optional human-readable name for the root.
	Name string `json:"name,omitempty"`
}

// ListRootsResult is the response to roots/list.
type ListRootsResult struct {
	Roots []Root `json:"roots"`
}
