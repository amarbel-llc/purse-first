# bob:rfc Skill

**Date:** 2026-02-28

## Problem

ADRs document decisions. FDRs document features. Neither is suited for
specifying interfaces — contracts that other code depends on and where precision
matters. A project-level RFC format adapted from IETF conventions fills this gap.

## Design

RFCs live in `docs/rfcs/` using `NNNN-title-with-dashes.md` naming. They use
RFC 2119 requirement keywords (MUST/SHOULD/MAY) for normative language and
follow a structure adapted from RFC 7322.

### Sections

| Section                | Required | Description                                          |
|------------------------|----------|------------------------------------------------------|
| Title (H1)            | Yes      | Interface name                                       |
| Abstract              | Yes      | Self-contained summary (2-4 sentences)               |
| Introduction          | Yes      | Problem context and scope                            |
| Requirements Language | Yes*     | RFC 2119 boilerplate (*when using keywords)          |
| Specification         | Yes      | The interface definition, normative                  |
| Security Considerations | Yes    | Security implications                                |
| Compatibility         | No       | Backwards compatibility, migration, versioning       |
| References            | No       | Normative and informative references                 |

### Differentiation

- **vs ADRs:** Normative (MUST/SHOULD/MAY) not descriptive. Specifies contracts,
  not decisions.
- **vs FDRs:** Aimed at implementers/consumers, not feature users. Includes
  Security Considerations.

### Deliverables

1. `skills/rfc/SKILL.md` — Skill document
2. `skills/rfc/references/rfc-template-bare.md` — Bare template
3. `.claude-plugin/plugin.json` — Register skill
