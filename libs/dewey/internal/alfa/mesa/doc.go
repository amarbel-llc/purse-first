// Package mesa renders tabular listings and (de)serializes them over the
// List-Table NDJSON protocol.
//
// A [Table] is built from column specs and rows of styled [Cell]s, then
// rendered: styled (colored, bordered) to a terminal, or plain
// TAB-separated text to a pipe. Styling is carried semantically as a fixed
// [Severity] vocabulary the renderer colors — producers never emit ANSI —
// so layout, width, and color live in one place across every consuming
// tool. The same [Table] serializes to an NDJSON stream (a header record
// then one record per row) via [EncodeStream] / [DecodeStream], which is
// how non-Go producers feed the renderer out-of-process.
//
// The wire contract is specified normatively in RFC 0003
// (docs/rfcs/0003-list-table-ndjson-protocol.md); the feature it serves is
// FDR 0015 (docs/features/0015-dewey-list-table-renderer.md).
package mesa
