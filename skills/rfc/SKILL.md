---
name: rfc
version: 0.1.0
description: Use when the user asks to "create an RFC", "write a spec", "specify an interface", "document a protocol", "add an interface spec", mentions "RFC", "specification", "wire format", "API contract", or is working in a docs/rfcs/ directory. Also applies when defining interfaces, protocols, file formats, or API contracts where precision and normative language (MUST/SHOULD/MAY) matter.
---

# Request for Comments

> **Self-contained examples.** All code and configuration below is complete and illustrative. Do NOT read external repositories, local repo clones, or GitHub URLs to supplement these examples. Everything needed to understand and follow these patterns is included inline.

A Request for Comments (RFC) specifies an interface — a contract that other code depends on. RFCs use normative language (MUST/SHOULD/MAY per RFC 2119) to precisely define behavior, making them suitable for protocols, wire formats, API contracts, and file format conventions. This format is adapted from IETF RFC conventions (RFC 7322 structure, RFC 2119 requirement keywords) for project-level use.

RFCs are distinct from ADRs and FDRs:

| | ADR | FDR | RFC |
|---|---|---|---|
| **Documents** | Why a choice was made | What a feature does | How an interface works |
| **Audience** | Future decision-makers | Feature users | Implementers and consumers |
| **Language** | Descriptive | Descriptive | Normative (MUST/SHOULD/MAY) |
| **Key sections** | Considered Options, Consequences | Motivation, Interface, Examples | Abstract, Specification, Security |

## When to Use

Create an RFC when:

- Defining a protocol or wire format (e.g., JSON-RPC message schemas, MCP tool interfaces)
- Specifying a file format convention (e.g., plugin.json structure, sweatfile format)
- Documenting an API contract that other packages depend on (e.g., library public interfaces)
- Any interface where breaking changes have consequences and precision matters

Do NOT create an RFC for:

- Architectural decisions (use an ADR)
- User-facing feature documentation (use an FDR)
- Internal implementation details not depended on by other code
- One-off scripts or throwaway interfaces

## RFC Structure

| Section | Required | Description |
|---------|----------|-------------|
| Title (H1) | Yes | Interface name, scannable in a file listing |
| Abstract | Yes | Self-contained summary of the interface (2-4 sentences, no references) |
| Introduction | Yes | Problem context and scope of this specification |
| Requirements Language | Yes* | RFC 2119 boilerplate (*required when using MUST/SHOULD/MAY) |
| Specification | Yes | The interface definition — precise, normative |
| Security Considerations | Yes | Security implications of the interface |
| Conformance Testing | Conditional | Conformance test suite via bats-emo (when a binary implements the spec) |
| Compatibility | No | Backwards compatibility, migration, versioning notes |
| References | No | Normative and informative references |

### Section Guidance

**Title** — Name the interface being specified. Good: "Plugin Manifest Format (plugin.json)". Bad: "Plugin spec" or "RFC about plugins".

**Abstract** — A self-contained summary that someone can read without any other context. Two to four sentences. Do not include references or citations. The abstract should answer: what is being specified, and why does it matter?

**Introduction** — Describe the problem that motivates the interface, the scope of the specification, and any relevant background. This is where you set context. Link to related ADRs, FDRs, or design documents for deeper background.

**Requirements Language** — When using RFC 2119 requirement keywords (MUST, SHOULD, MAY, etc.), include this exact boilerplate:

> The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.

Requirement keywords MUST be uppercase when used in their normative sense. When using these words in their ordinary English sense (e.g., "the system must be installed first"), use lowercase to avoid confusion.

**Specification** — The core of the document. Define the interface precisely using normative language. Structure this section with subsections as needed. Include:

- Data formats (schemas, field definitions, types)
- Behavioral requirements (what implementations MUST/SHOULD/MAY do)
- Error handling (how failures MUST be reported)
- Examples (concrete instances of valid and invalid data)

Be precise. "The `name` field MUST be a non-empty string" is better than "the name field should have a value."

**Security Considerations** — Document security implications of the interface. Every RFC MUST include this section, even if only to state that the interface has no security implications. Consider: authentication, authorization, data validation, injection risks, trust boundaries, and information disclosure.

**Compatibility** — Document backwards compatibility constraints, migration paths, and versioning strategy. Include this section when the interface has existing consumers or when defining how future versions will be handled.

**References** — Split into Normative (required for conformance) and Informative (background reading) when both types exist. Use bracketed citation format: `[RFC 2119]`, `[plugin.json v1]`.

## File and Directory Conventions

- **Directory:** `docs/rfcs/`
- **Naming pattern:** `NNNN-title-with-dashes.md` (e.g., `0001-plugin-manifest-format.md`)
- **Numbering:** Sequential, zero-padded to 4 digits
- **Casing:** Lowercase throughout, dashes for word separation
- **One interface per file** — never combine multiple specifications

When creating the first RFC in a project, create the `docs/rfcs/` directory and start numbering at `0001`. To determine the next number, find the highest-numbered existing file and increment by one.

## Template

When creating an RFC, read the bare template from the `references/` directory, fill in the sections, remove any optional sections that do not apply, and save the result in `docs/rfcs/` with the appropriate sequential number.

- **`references/rfc-template-bare.md`** — All sections, no guidance — fill in directly

## Metadata

RFCs use YAML front matter for status tracking:

```yaml
---
status: proposed
date: 2026-02-28
---
```

| Field | Values / Format | Purpose |
|-------|----------------|---------|
| `status` | `exploring` &#124; `proposed` &#124; `experimental` &#124; `testing` &#124; `accepted` &#124; `deprecated` &#124; `superseded by RFC-NNNN` | Current state of the specification |
| `date` | `YYYY-MM-DD` | Date the record was last updated |

## Status Lifecycle

RFC status progresses through these transitions:

- `exploring` — Problem defined, collecting thoughts on potential interface approaches.
- `exploring` --> `proposed` — Interface design selected, full RFC drafted.
- `proposed` --> `accepted` — The specification is ratified and implementations should conform to it.
- `accepted` --> `experimental` — Specification implemented in limited scope, not yet validated.
- `experimental` --> `testing` — Promotion criteria defined and being measured.
- `testing` --> `accepted` — Promotion criteria met, specification fully validated.
- `accepted` --> `deprecated` — The interface is no longer supported (e.g., the protocol has been retired).
- `accepted` --> `superseded by RFC-NNNN` — The specification is replaced by a newer version.

When superseding an RFC:

1. Update the old RFC's status to `superseded by RFC-NNNN`
2. In the new RFC's **References** section, cite the old RFC as normative
3. In the new RFC's **Compatibility** section, describe migration from the old specification
4. Both RFCs should cross-reference each other for traceability

## Requirement Keywords Reference

Adapted from RFC 2119. Use these keywords only when specifying behavior that matters for conformance:

| Keyword | Meaning |
|---------|---------|
| **MUST** / **REQUIRED** / **SHALL** | Absolute requirement. Non-conformant if violated. |
| **MUST NOT** / **SHALL NOT** | Absolute prohibition. Non-conformant if violated. |
| **SHOULD** / **RECOMMENDED** | May be ignored with valid reason, but implications must be understood. |
| **SHOULD NOT** / **NOT RECOMMENDED** | May be done with valid reason, but implications must be understood. |
| **MAY** / **OPTIONAL** | Truly optional. Implementations may or may not include this. |

Use restraint. Not every sentence needs a requirement keyword. Reserve them for behavior that affects interoperability, conformance, or safety.

## Writing Tips

- **Be precise, not verbose.** "The `version` field MUST be a string matching `^[0-9]+\\.[0-9]+\\.[0-9]+$`" is better than a paragraph explaining version formats.
- **Use MUST for invariants, SHOULD for best practices, MAY for optional behavior.** Overusing MUST makes specifications brittle. Overusing MAY makes them meaningless.
- **Include examples in the Specification section.** Show valid and invalid instances of the data format. Concrete examples catch ambiguities that prose misses.
- **Abstract must stand alone.** Someone reading only the abstract should understand what is being specified and why.
- **Security Considerations is always required.** Even if the interface has no security implications, state that explicitly. The section's presence forces you to think about it.
- **Write for implementers.** The primary reader is someone building a conformant implementation. Every requirement should be testable — if you can't write a test for it, it's not precise enough.
- **Separate normative from informative.** Requirement keywords (uppercase MUST/SHOULD/MAY) define the contract. Everything else is explanatory context.
- **Do not backfill retroactively unless asked.** Only create RFCs for interfaces being defined now.
- **Remove unused optional sections.** If Compatibility or References do not apply, delete them.

## Conformance Tests

When the protocol being specified describes behavior that can reasonably be tested against a specific binary — CLI tools, MCP servers, file processors, or any executable that consumes or produces the specified format — the RFC MUST include a `Conformance Testing` section.

This section specifies:

1. **What to test**: Which normative requirements (MUST/MUST NOT) are covered by conformance tests
2. **Where tests live**: Path to a `zz-tests_bats/` directory containing the conformance suite
3. **Binary injection**: Tests MUST use the `bats-emo` library (`require_bin`) for binary injection — never hardcode build output paths. This makes the suite portable across implementations (e.g., a Go binary and a Rust rewrite can run the same tests)

Use the `robin:bats-testing` skill for test structure and the `robin:emo` skill for binary injection patterns.

### When to Include

Include this section when:

- The RFC specifies CLI behavior (flags, exit codes, stdout/stderr format)
- The RFC specifies a wire format that a binary produces or consumes (JSON-RPC, TAP, etc.)
- The RFC specifies file format handling where a tool reads/writes the format
- Any MUST requirement can be verified by running a binary and checking its output

Do NOT include this section when:

- The RFC specifies a purely declarative format with no reference implementation binary (e.g., a JSON schema convention)
- The protocol is between two libraries with no CLI surface
- No binary exists or is planned that implements the specification

### Template Addition

When the conformance testing section applies, add this to the RFC after Security Considerations:

```markdown
## Conformance Testing

Conformance tests for this specification live in `<path>/zz-tests_bats/`.

Tests use binary injection via `bats-emo`:

    require_bin <VAR_NAME> <command-name>

### Covered Requirements

| Requirement | Test File | Description |
|-------------|-----------|-------------|
| Section N.N, MUST ... | `test_file.bats` | What the test verifies |
```

## Related Skills

- **bob:adr** — Architecture Decision Records for documenting choices and trade-offs
- **bob:fdr** — Feature Design Records for documenting user-facing features
- **bob:overview** — Framework orientation, terminology, and workflow overview
- **robin:bats-testing** — BATS test structure, helpers, and integration patterns
- **robin:emo** — Binary injection via `require_bin` for conformance test portability
