use regex::Regex;
use std::sync::LazyLock;
use thiserror::Error;

#[derive(Error, Debug)]
pub enum ValidationError {
    #[error("invalid flake reference: `{0}`")]
    InvalidFlakeRef(String),

    #[error("shell metacharacters not allowed: `{0}`. Retry with the metacharacters removed — use separate array entries or tool parameters instead of shell operators")]
    ShellMetacharacters(String),

    #[error("nix expression contains invalid characters (null bytes): `{0}`")]
    InvalidNixExpr(String),

    #[error("invalid attribute path: `{0}`")]
    InvalidAttrPath(String),

    #[error("invalid cache name: `{0}`")]
    InvalidCacheName(String),

    #[error("invalid store path: `{0}`")]
    InvalidStorePath(String),

    #[error("invalid store subpath: `{0}`")]
    InvalidStoreSubpath(String),

    #[error("invalid path: `{0}`")]
    InvalidPath(String),
}

static FLAKE_REF_PATTERN: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"^[a-zA-Z0-9._\-/:#+]+$").unwrap());

static ATTR_PATH_PATTERN: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"^[a-zA-Z0-9._\-]+$").unwrap());

static SHELL_METACHARACTERS: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"[;&|`$(){}\\<>!]").unwrap());

// Cachix cache names: alphanumeric with hyphens, must start with alphanumeric
static CACHE_NAME_PATTERN: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"^[a-zA-Z0-9][a-zA-Z0-9\-]*$").unwrap());

// Nix store paths: /nix/store/<32-char-hash>-<name>
static STORE_PATH_PATTERN: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"^/nix/store/[a-z0-9]{32}-[a-zA-Z0-9._\-]+$").unwrap());

// Nix store subpaths: /nix/store/<32-char-hash>-<name>[/<sub-path>]
// Allows dotfiles; . and .. are rejected programmatically in validate_store_subpath
static STORE_SUBPATH_PATTERN: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"^/nix/store/[a-z0-9]{32}-[a-zA-Z0-9._\-]+(/[a-zA-Z0-9._\-]+)*$").unwrap()
});

// File paths: no shell metacharacters, reasonable characters
static PATH_PATTERN: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"^[a-zA-Z0-9._\-/~]+$").unwrap());

pub fn validate_installable(installable: &str) -> Result<&str, ValidationError> {
    if !FLAKE_REF_PATTERN.is_match(installable) {
        return Err(ValidationError::InvalidFlakeRef(installable.to_string()));
    }
    Ok(installable)
}

pub fn validate_flake_ref(flake_ref: &str) -> Result<&str, ValidationError> {
    if !FLAKE_REF_PATTERN.is_match(flake_ref) {
        return Err(ValidationError::InvalidFlakeRef(flake_ref.to_string()));
    }
    Ok(flake_ref)
}

pub fn validate_attr_path(attr_path: &str) -> Result<&str, ValidationError> {
    if !ATTR_PATH_PATTERN.is_match(attr_path) {
        return Err(ValidationError::InvalidAttrPath(attr_path.to_string()));
    }
    Ok(attr_path)
}

pub fn validate_no_shell_metacharacters(input: &str) -> Result<&str, ValidationError> {
    if SHELL_METACHARACTERS.is_match(input) {
        return Err(ValidationError::ShellMetacharacters(input.to_string()));
    }
    Ok(input)
}

pub fn validate_args(args: &[String]) -> Result<(), ValidationError> {
    for arg in args {
        validate_no_shell_metacharacters(arg)?;
    }
    Ok(())
}

/// Validate a Nix expression for use with `nix eval --expr` or `--apply`.
///
/// Since commands are spawned via `tokio::process::Command` (direct execvp,
/// no shell), Nix syntax characters like `$`, `{}`, `()` are safe — they are
/// passed as literal bytes to the nix process. Only null bytes are rejected
/// as they cannot be passed in argv.
pub fn validate_nix_expr(input: &str) -> Result<&str, ValidationError> {
    if input.contains('\0') {
        return Err(ValidationError::InvalidNixExpr(input.to_string()));
    }
    Ok(input)
}

pub fn validate_cache_name(name: &str) -> Result<&str, ValidationError> {
    if !CACHE_NAME_PATTERN.is_match(name) {
        return Err(ValidationError::InvalidCacheName(name.to_string()));
    }
    Ok(name)
}

pub fn validate_store_path(path: &str) -> Result<&str, ValidationError> {
    if !STORE_PATH_PATTERN.is_match(path) {
        return Err(ValidationError::InvalidStorePath(path.to_string()));
    }
    Ok(path)
}

pub fn validate_store_subpath(path: &str) -> Result<&str, ValidationError> {
    if !STORE_SUBPATH_PATTERN.is_match(path) {
        return Err(ValidationError::InvalidStoreSubpath(path.to_string()));
    }
    if path.split('/').any(|c| c == "." || c == "..") {
        return Err(ValidationError::InvalidStoreSubpath(path.to_string()));
    }
    Ok(path)
}

pub fn validate_store_paths(paths: &[String]) -> Result<(), ValidationError> {
    for path in paths {
        validate_store_path(path)?;
    }
    Ok(())
}

pub fn validate_path(path: &str) -> Result<&str, ValidationError> {
    if !PATH_PATTERN.is_match(path) {
        return Err(ValidationError::InvalidPath(path.to_string()));
    }
    Ok(path)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_valid_installables() {
        assert!(validate_installable(".#default").is_ok());
        assert!(validate_installable("nixpkgs#hello").is_ok());
        assert!(validate_installable("github:NixOS/nixpkgs#hello").is_ok());
        assert!(validate_installable(".#packages.x86_64-linux.default").is_ok());
    }

    #[test]
    fn test_invalid_installables() {
        assert!(validate_installable("$(malicious)").is_err());
        assert!(validate_installable("; rm -rf /").is_err());
        assert!(validate_installable("hello`whoami`").is_err());
    }

    #[test]
    fn test_shell_metacharacters() {
        assert!(validate_no_shell_metacharacters("hello").is_ok());
        assert!(validate_no_shell_metacharacters("hello; rm -rf").is_err());
        assert!(validate_no_shell_metacharacters("$(cmd)").is_err());
        assert!(validate_no_shell_metacharacters("foo | bar").is_err());
    }

    #[test]
    fn test_nix_expr() {
        // Valid Nix expressions with syntax characters
        assert!(validate_nix_expr("{ x = 1; }").is_ok());
        assert!(validate_nix_expr("builtins.attrNames { a = 1; b = 2; }").is_ok());
        assert!(validate_nix_expr("let x = 1; in x").is_ok());
        assert!(validate_nix_expr("(import <nixpkgs> {}).hello").is_ok());
        assert!(validate_nix_expr("\"hello ${name}\"").is_ok());
        assert!(validate_nix_expr("x: x + 1").is_ok());
        assert!(validate_nix_expr("a && b || c").is_ok());
        assert!(validate_nix_expr("if true then 1 else 2").is_ok());
        assert!(validate_nix_expr("map (x: x * 2) [1 2 3]").is_ok());
        // Null bytes are rejected
        assert!(validate_nix_expr("hello\0world").is_err());
    }

    #[test]
    fn test_cache_name() {
        assert!(validate_cache_name("mycache").is_ok());
        assert!(validate_cache_name("my-cache").is_ok());
        assert!(validate_cache_name("cache123").is_ok());
        assert!(validate_cache_name("-invalid").is_err());
        assert!(validate_cache_name("").is_err());
        assert!(validate_cache_name("cache;injection").is_err());
    }

    #[test]
    fn test_store_path() {
        assert!(validate_store_path(
            "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-hello"
        )
        .is_ok());
        assert!(validate_store_path(
            "/nix/store/abcdefghijklmnopqrstuvwxyz012345-package-1.0"
        )
        .is_ok());
        assert!(validate_store_path("/tmp/not-store").is_err());
        assert!(validate_store_path("/nix/store/short-hash").is_err());
    }

    #[test]
    fn test_store_subpath() {
        assert!(validate_store_subpath(
            "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-hello"
        )
        .is_ok());
        assert!(validate_store_subpath(
            "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-hello/bin/hello"
        )
        .is_ok());
        assert!(validate_store_subpath(
            "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-package-1.0/share/doc"
        )
        .is_ok());
        assert!(validate_store_subpath("/tmp/not-store").is_err());
        assert!(validate_store_subpath(
            "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-hello/../../etc/passwd"
        )
        .is_err());
        assert!(validate_store_subpath(
            "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-hello/bin;whoami"
        )
        .is_err());
    }

    #[test]
    fn test_store_subpath_dotfiles() {
        assert!(validate_store_subpath(
            "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-pkg/.claude-plugin"
        )
        .is_ok());
        assert!(validate_store_subpath(
            "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-pkg/.config/settings"
        )
        .is_ok());
        assert!(validate_store_subpath(
            "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-pkg/.hidden-dir/.hidden-file"
        )
        .is_ok());
        assert!(validate_store_subpath(
            "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-pkg/."
        )
        .is_err());
        assert!(validate_store_subpath(
            "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-pkg/.."
        )
        .is_err());
        assert!(validate_store_subpath(
            "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-pkg/./bin"
        )
        .is_err());
        assert!(validate_store_subpath(
            "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-pkg/../other"
        )
        .is_err());
    }

    #[test]
    fn test_path() {
        assert!(validate_path("/home/user/result").is_ok());
        assert!(validate_path("./result").is_ok());
        assert!(validate_path("~/project/result").is_ok());
        assert!(validate_path("/path;injection").is_err());
        assert!(validate_path("/path$(cmd)").is_err());
    }
}
