use serde::{Deserialize, Serialize};

/// Icon represents a visual icon for display in user interfaces.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Icon {
    /// URI of the icon image.
    pub src: String,

    /// Override MIME type of the icon.
    #[serde(rename = "mimeType", skip_serializing_if = "Option::is_none")]
    pub mime_type: Option<String>,

    /// Available sizes in "WxH" format or "any".
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sizes: Option<Vec<String>>,

    /// Intended display theme ("light" or "dark").
    #[serde(skip_serializing_if = "Option::is_none")]
    pub theme: Option<String>,
}
