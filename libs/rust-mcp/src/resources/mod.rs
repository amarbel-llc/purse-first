pub mod handler;
pub mod handler_v1;
pub mod registry;

pub use handler::{Resource, ResourceContent, ResourceError, ResourceInfo};
pub use handler_v1::{ResourceInfoV1, ResourceV1};
pub use registry::ResourceRegistry;
