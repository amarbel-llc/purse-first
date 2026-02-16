// Validation utilities - applications can use these for input validation
// but they're not enforced by the library

use thiserror::Error;

#[derive(Error, Debug)]
pub enum ValidationError {
    #[error("Invalid input: {0}")]
    Invalid(String),

    #[error("Pattern match failed: {0}")]
    PatternMismatch(String),
}

/// Validate that a string doesn't contain shell metacharacters
pub fn validate_no_shell_metacharacters(input: &str) -> Result<&str, ValidationError> {
    let dangerous_chars = [';', '|', '&', '$', '(', ')', '{', '}', '\\', '<', '>', '!', '`'];

    for ch in dangerous_chars {
        if input.contains(ch) {
            return Err(ValidationError::Invalid(format!(
                "Input contains dangerous character: {}",
                ch
            )));
        }
    }

    Ok(input)
}
