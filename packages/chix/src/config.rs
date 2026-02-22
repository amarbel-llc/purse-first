use serde::Deserialize;
use std::collections::HashMap;
use std::env;
use std::fs;
use std::path::PathBuf;

use crate::output::OutputLimitsConfig;

#[derive(Debug, Default, Deserialize)]
pub struct Config {
    #[serde(default)]
    pub cachix: CachixConfig,
    #[serde(default)]
    pub flakehub: FlakehubConfig,
    #[serde(default)]
    pub output_limits: OutputLimitsConfig,
}

#[derive(Debug, Default, Deserialize)]
pub struct CachixConfig {
    pub default_cache: Option<String>,
    pub auth_token: Option<String>,
    #[serde(default)]
    pub caches: HashMap<String, CacheEntry>,
}

#[derive(Debug, Default, Deserialize)]
pub struct CacheEntry {
    pub auth_token: Option<String>,
}

#[derive(Debug, Default, Deserialize)]
pub struct FlakehubConfig {
    // FlakeHub uses netrc-based auth managed by 'fh login'
    // This section is for future configuration options
}

fn config_path() -> Option<PathBuf> {
    dirs::config_dir().map(|d| d.join("nix-mcp-server").join("config.toml"))
}

pub fn load_config() -> Config {
    let Some(path) = config_path() else {
        return Config::default();
    };

    if !path.exists() {
        return Config::default();
    }

    match fs::read_to_string(&path) {
        Ok(contents) => match toml::from_str(&contents) {
            Ok(config) => config,
            Err(e) => {
                eprintln!("Warning: failed to parse config at {:?}: {}", path, e);
                Config::default()
            }
        },
        Err(e) => {
            eprintln!("Warning: failed to read config at {:?}: {}", path, e);
            Config::default()
        }
    }
}

pub fn get_cachix_token(config: &Config, cache_name: Option<&str>) -> Option<String> {
    // Priority:
    // 1. Per-cache token from config
    // 2. Global token from config
    // 3. CACHIX_AUTH_TOKEN env var

    if let Some(name) = cache_name {
        if let Some(entry) = config.cachix.caches.get(name) {
            if let Some(ref token) = entry.auth_token {
                return Some(token.clone());
            }
        }
    }

    if let Some(ref token) = config.cachix.auth_token {
        return Some(token.clone());
    }

    env::var("CACHIX_AUTH_TOKEN").ok()
}

pub fn get_default_cache(config: &Config) -> Option<String> {
    config.cachix.default_cache.clone()
}

pub fn get_output_limits_config(config: &Config) -> &OutputLimitsConfig {
    &config.output_limits
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_empty_config() {
        let config = Config::default();
        assert!(config.cachix.default_cache.is_none());
        assert!(config.cachix.auth_token.is_none());
    }

    #[test]
    fn test_parse_config() {
        let toml_str = r#"
[cachix]
default_cache = "mycache"
auth_token = "secret-token"

[cachix.caches.work]
auth_token = "work-token"
"#;
        let config: Config = toml::from_str(toml_str).unwrap();
        assert_eq!(config.cachix.default_cache, Some("mycache".to_string()));
        assert_eq!(config.cachix.auth_token, Some("secret-token".to_string()));
        assert_eq!(
            config.cachix.caches.get("work").unwrap().auth_token,
            Some("work-token".to_string())
        );
    }

    #[test]
    fn test_get_cachix_token_priority() {
        let toml_str = r#"
[cachix]
auth_token = "global-token"

[cachix.caches.specific]
auth_token = "specific-token"
"#;
        let config: Config = toml::from_str(toml_str).unwrap();

        // Per-cache token takes priority
        assert_eq!(
            get_cachix_token(&config, Some("specific")),
            Some("specific-token".to_string())
        );

        // Falls back to global token
        assert_eq!(
            get_cachix_token(&config, Some("other")),
            Some("global-token".to_string())
        );

        // No cache name uses global
        assert_eq!(
            get_cachix_token(&config, None),
            Some("global-token".to_string())
        );
    }

    #[test]
    fn test_output_limits_config() {
        let toml_str = r#"
[output_limits]
default_max_bytes = 50000
default_max_lines = 1000
default_max_items = 50
log_tail_default = 250
search_limit_default = 25
"#;
        let config: Config = toml::from_str(toml_str).unwrap();

        assert_eq!(config.output_limits.default_max_bytes(), 50000);
        assert_eq!(config.output_limits.default_max_lines(), 1000);
        assert_eq!(config.output_limits.default_max_items(), 50);
        assert_eq!(config.output_limits.log_tail_default(), 250);
        assert_eq!(config.output_limits.search_limit_default(), 25);
    }

    #[test]
    fn test_output_limits_defaults() {
        let config = Config::default();
        // Should use hardcoded defaults when not specified
        assert_eq!(config.output_limits.default_max_bytes(), 100_000);
        assert_eq!(config.output_limits.default_max_lines(), 2000);
        assert_eq!(config.output_limits.default_max_items(), 100);
        assert_eq!(config.output_limits.log_tail_default(), 500);
        assert_eq!(config.output_limits.search_limit_default(), 50);
    }
}
