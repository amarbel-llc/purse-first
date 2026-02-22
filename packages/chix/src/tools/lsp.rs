use crate::lsp_client::{create_nil_client, LspClient};
use crate::output::PaginationInfo;
use crate::validators::validate_no_shell_metacharacters;
use serde::Serialize;
use std::path::Path;

#[derive(Debug, Serialize)]
pub struct DiagnosticsResult {
    pub success: bool,
    pub file_path: String,
    pub diagnostics: Vec<DiagnosticInfo>,
    pub error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pagination: Option<PaginationInfo>,
}

#[derive(Debug, Serialize)]
pub struct DiagnosticInfo {
    pub line: u32,
    pub character: u32,
    pub end_line: u32,
    pub end_character: u32,
    pub severity: String,
    pub message: String,
    pub source: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct CompletionsResult {
    pub success: bool,
    pub completions: Vec<CompletionInfo>,
    pub error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pagination: Option<PaginationInfo>,
}

#[derive(Debug, Serialize)]
pub struct CompletionInfo {
    pub label: String,
    pub kind: Option<String>,
    pub detail: Option<String>,
    pub documentation: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct HoverInfoResult {
    pub success: bool,
    pub contents: Option<String>,
    pub range: Option<RangeInfo>,
    pub error: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct RangeInfo {
    pub start_line: u32,
    pub start_character: u32,
    pub end_line: u32,
    pub end_character: u32,
}

#[derive(Debug, Serialize)]
pub struct DefinitionResult {
    pub success: bool,
    pub locations: Vec<LocationInfo>,
    pub error: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct LocationInfo {
    pub uri: String,
    pub line: u32,
    pub character: u32,
    pub end_line: u32,
    pub end_character: u32,
}

fn completion_kind_to_string(kind: u32) -> &'static str {
    match kind {
        1 => "text",
        2 => "method",
        3 => "function",
        4 => "constructor",
        5 => "field",
        6 => "variable",
        7 => "class",
        8 => "interface",
        9 => "module",
        10 => "property",
        11 => "unit",
        12 => "value",
        13 => "enum",
        14 => "keyword",
        15 => "snippet",
        16 => "color",
        17 => "file",
        18 => "reference",
        19 => "folder",
        20 => "enum_member",
        21 => "constant",
        22 => "struct",
        23 => "event",
        24 => "operator",
        25 => "type_parameter",
        _ => "unknown",
    }
}

fn file_to_uri(path: &str) -> String {
    if path.starts_with("file://") {
        path.to_string()
    } else {
        format!("file://{}", path)
    }
}

fn get_root_uri(file_path: &str) -> Option<String> {
    Path::new(file_path)
        .parent()
        .map(|p| format!("file://{}", p.display()))
}

async fn read_file_contents(path: &str) -> Result<String, String> {
    tokio::fs::read_to_string(path)
        .await
        .map_err(|e| format!("Failed to read file: {}", e))
}

pub async fn nil_diagnostics(
    file_path: String,
    offset: Option<usize>,
    limit: Option<usize>,
) -> Result<DiagnosticsResult, String> {
    validate_no_shell_metacharacters(&file_path).map_err(|e| e.to_string())?;

    let path = Path::new(&file_path);
    if !path.exists() {
        return Ok(DiagnosticsResult {
            success: false,
            file_path,
            diagnostics: vec![],
            error: Some("File not found".to_string()),
            pagination: None,
        });
    }

    let contents = read_file_contents(&file_path).await?;
    let uri = file_to_uri(&file_path);
    let root_uri = get_root_uri(&file_path);

    let mut client = create_nil_client().await.map_err(|e| e.to_string())?;

    client
        .initialize(root_uri.as_deref())
        .await
        .map_err(|e| e.to_string())?;

    client
        .did_open(&uri, &contents)
        .await
        .map_err(|e| e.to_string())?;

    let diagnostics = client.diagnostics(&uri).await.map_err(|e| e.to_string())?;

    let _ = client.shutdown().await;

    let all_infos: Vec<DiagnosticInfo> = diagnostics
        .into_iter()
        .map(|d| DiagnosticInfo {
            line: d.range.start.line,
            character: d.range.start.character,
            end_line: d.range.end.line,
            end_character: d.range.end.character,
            severity: d.severity.map(|s| s.as_str()).unwrap_or("unknown").to_string(),
            message: d.message,
            source: d.source,
        })
        .collect();

    let total = all_infos.len();
    let off = offset.unwrap_or(0);
    let lim = limit.unwrap_or(total);

    let paginated: Vec<DiagnosticInfo> = all_infos.into_iter().skip(off).take(lim).collect();
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

    Ok(DiagnosticsResult {
        success: true,
        file_path,
        diagnostics: paginated,
        error: None,
        pagination,
    })
}

pub async fn nil_completions(
    file_path: String,
    line: u32,
    character: u32,
    offset: Option<usize>,
    limit: Option<usize>,
) -> Result<CompletionsResult, String> {
    validate_no_shell_metacharacters(&file_path).map_err(|e| e.to_string())?;

    let path = Path::new(&file_path);
    if !path.exists() {
        return Ok(CompletionsResult {
            success: false,
            completions: vec![],
            error: Some("File not found".to_string()),
            pagination: None,
        });
    }

    let contents = read_file_contents(&file_path).await?;
    let uri = file_to_uri(&file_path);
    let root_uri = get_root_uri(&file_path);

    let mut client = create_nil_client().await.map_err(|e| e.to_string())?;

    client
        .initialize(root_uri.as_deref())
        .await
        .map_err(|e| e.to_string())?;

    client
        .did_open(&uri, &contents)
        .await
        .map_err(|e| e.to_string())?;

    let completions = client
        .completion(&uri, line, character)
        .await
        .map_err(|e| e.to_string())?;

    let _ = client.shutdown().await;

    let all_infos: Vec<CompletionInfo> = completions
        .into_iter()
        .map(|c| CompletionInfo {
            label: c.label,
            kind: c.kind.map(completion_kind_to_string).map(|s| s.to_string()),
            detail: c.detail,
            documentation: c.documentation,
        })
        .collect();

    let total = all_infos.len();
    let off = offset.unwrap_or(0);
    let lim = limit.unwrap_or(total);

    let paginated: Vec<CompletionInfo> = all_infos.into_iter().skip(off).take(lim).collect();
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

    Ok(CompletionsResult {
        success: true,
        completions: paginated,
        error: None,
        pagination,
    })
}

pub async fn nil_hover(
    file_path: String,
    line: u32,
    character: u32,
) -> Result<HoverInfoResult, String> {
    validate_no_shell_metacharacters(&file_path).map_err(|e| e.to_string())?;

    let path = Path::new(&file_path);
    if !path.exists() {
        return Ok(HoverInfoResult {
            success: false,
            contents: None,
            range: None,
            error: Some("File not found".to_string()),
        });
    }

    let contents = read_file_contents(&file_path).await?;
    let uri = file_to_uri(&file_path);
    let root_uri = get_root_uri(&file_path);

    let mut client = create_nil_client().await.map_err(|e| e.to_string())?;

    client
        .initialize(root_uri.as_deref())
        .await
        .map_err(|e| e.to_string())?;

    client
        .did_open(&uri, &contents)
        .await
        .map_err(|e| e.to_string())?;

    let hover = client
        .hover(&uri, line, character)
        .await
        .map_err(|e| e.to_string())?;

    let _ = client.shutdown().await;

    Ok(HoverInfoResult {
        success: true,
        contents: hover.as_ref().map(|h| h.contents.clone()),
        range: hover.and_then(|h| {
            h.range.map(|r| RangeInfo {
                start_line: r.start.line,
                start_character: r.start.character,
                end_line: r.end.line,
                end_character: r.end.character,
            })
        }),
        error: None,
    })
}

pub async fn nil_definition(
    file_path: String,
    line: u32,
    character: u32,
) -> Result<DefinitionResult, String> {
    validate_no_shell_metacharacters(&file_path).map_err(|e| e.to_string())?;

    let path = Path::new(&file_path);
    if !path.exists() {
        return Ok(DefinitionResult {
            success: false,
            locations: vec![],
            error: Some("File not found".to_string()),
        });
    }

    let contents = read_file_contents(&file_path).await?;
    let uri = file_to_uri(&file_path);
    let root_uri = get_root_uri(&file_path);

    let mut client = create_nil_client().await.map_err(|e| e.to_string())?;

    client
        .initialize(root_uri.as_deref())
        .await
        .map_err(|e| e.to_string())?;

    client
        .did_open(&uri, &contents)
        .await
        .map_err(|e| e.to_string())?;

    let locations = client
        .goto_definition(&uri, line, character)
        .await
        .map_err(|e| e.to_string())?;

    let _ = client.shutdown().await;

    let location_infos: Vec<LocationInfo> = locations
        .into_iter()
        .map(|l| LocationInfo {
            uri: l.uri,
            line: l.range.start.line,
            character: l.range.start.character,
            end_line: l.range.end.line,
            end_character: l.range.end.character,
        })
        .collect();

    Ok(DefinitionResult {
        success: true,
        locations: location_infos,
        error: None,
    })
}
