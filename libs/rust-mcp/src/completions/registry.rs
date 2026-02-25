use super::handler::{CompletionError, CompletionProvider};
use crate::protocol::completions::{CompletionCompleteParams, CompletionResult, CompletionValues};
use crate::server::Context;
use std::sync::Arc;

pub struct CompletionRegistry {
    provider: Option<Arc<dyn CompletionProvider>>,
}

impl CompletionRegistry {
    pub fn new() -> Self {
        CompletionRegistry { provider: None }
    }

    pub fn register<P: CompletionProvider + 'static>(&mut self, provider: P) {
        self.provider = Some(Arc::new(provider));
    }

    pub fn has_provider(&self) -> bool {
        self.provider.is_some()
    }

    pub async fn complete(
        &self,
        params: CompletionCompleteParams,
        ctx: &Context,
    ) -> Result<CompletionResult, CompletionError> {
        match &self.provider {
            Some(provider) => provider.complete(params, ctx).await,
            None => Ok(CompletionResult {
                completion: CompletionValues {
                    values: vec![],
                    total: None,
                    has_more: None,
                },
                meta: None,
            }),
        }
    }
}

impl Default for CompletionRegistry {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::protocol::completions::*;
    use crate::server::Context;

    struct TestProvider;

    #[async_trait::async_trait]
    impl CompletionProvider for TestProvider {
        async fn complete(
            &self,
            params: CompletionCompleteParams,
            _ctx: &Context,
        ) -> Result<CompletionResult, CompletionError> {
            let prefix = &params.argument.value;
            let values: Vec<String> = vec!["foo", "foobar", "baz"]
                .into_iter()
                .filter(|v| v.starts_with(prefix))
                .map(String::from)
                .collect();

            Ok(CompletionResult {
                completion: CompletionValues {
                    values,
                    total: None,
                    has_more: None,
                },
                meta: None,
            })
        }
    }

    #[tokio::test]
    async fn registry_delegates_to_provider() {
        let mut registry = CompletionRegistry::new();
        registry.register(TestProvider);

        let params = CompletionCompleteParams {
            r#ref: CompletionReference {
                ref_type: "ref/prompt".to_string(),
                name: Some("test".to_string()),
                uri: None,
            },
            argument: CompletionArgument {
                name: "arg".to_string(),
                value: "fo".to_string(),
            },
            context: None,
        };

        let ctx = Context::new("test-server", "0.1.0", Default::default());
        let result = registry.complete(params, &ctx).await.unwrap();
        assert_eq!(result.completion.values, vec!["foo", "foobar"]);
    }

    #[tokio::test]
    async fn registry_empty_returns_empty() {
        let registry = CompletionRegistry::new();

        let params = CompletionCompleteParams {
            r#ref: CompletionReference {
                ref_type: "ref/prompt".to_string(),
                name: Some("missing".to_string()),
                uri: None,
            },
            argument: CompletionArgument {
                name: "arg".to_string(),
                value: "x".to_string(),
            },
            context: None,
        };

        let ctx = Context::new("test-server", "0.1.0", Default::default());
        let result = registry.complete(params, &ctx).await.unwrap();
        assert!(result.completion.values.is_empty());
    }
}
