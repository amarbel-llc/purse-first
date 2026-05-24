# primordial

Fundamental system-level utilities.

## Functions

- `IsTTY(f)`: Check if file descriptor is a terminal (TTY)
- `IsTty(f)`: Deprecated alias for `IsTTY`

Uses golang.org/x/term for cross-platform terminal detection.
Used for conditional output formatting based on terminal capabilities.
