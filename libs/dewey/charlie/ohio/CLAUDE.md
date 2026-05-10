# ohio

I/O streaming utilities with piped readers.

## Key Types

- `PipedReader`: Interface for piped reading with close
- `PipedWriter`: Symmetric mirror — producers Write, consumers WriteTo

## Features

- Piped reader/writer construction
- Async read-from operations
- Error propagation back to producer when consumer's writer fails
