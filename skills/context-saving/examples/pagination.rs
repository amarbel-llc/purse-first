// Minimal pagination example for an MCP tool returning a list of items.
//
// This shows the core pattern: collect all items, apply skip/take,
// conditionally include PaginationInfo.

use serde::Serialize;

#[derive(Debug, Serialize)]
pub struct PaginationInfo {
    pub offset: usize,
    pub limit: usize,
    pub total: usize,
    pub has_more: bool,
}

#[derive(Debug, Serialize)]
pub struct ListResult {
    pub items: Vec<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pagination: Option<PaginationInfo>,
}

pub fn list_items(
    all_items: Vec<String>,
    offset: Option<usize>,
    limit: Option<usize>,
) -> ListResult {
    let total = all_items.len();
    let off = offset.unwrap_or(0);
    let lim = limit.unwrap_or(total);

    let paginated: Vec<String> = all_items.into_iter().skip(off).take(lim).collect();
    let kept_count = paginated.len();
    let has_more = off + kept_count < total;

    let pagination = if offset.is_some() || limit.is_some() {
        Some(PaginationInfo {
            offset: off,
            limit: lim,
            total,
            has_more,
        })
    } else {
        None
    };

    ListResult {
        items: paginated,
        pagination,
    }
}
