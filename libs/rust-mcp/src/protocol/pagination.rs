use serde::{Deserialize, Serialize};

/// Cursor-based pagination parameters.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PaginationParams {
    /// Opaque pagination token from a previous response.
    #[serde(default)]
    pub cursor: Option<String>,
}

/// Pagination result with next cursor.
#[derive(Debug, Clone, Serialize)]
pub struct PaginatedResult {
    /// Opaque token for the next page.
    #[serde(rename = "nextCursor", skip_serializing_if = "Option::is_none")]
    pub next_cursor: Option<String>,
}
