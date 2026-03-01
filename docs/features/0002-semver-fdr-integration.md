---
status: exploring
date: 2026-03-01
promotion-criteria:
---

# Semver / FDR Lifecycle Integration

## Problem Statement

Packages have no versioning convention, and there's no connection between feature
lifecycle (FDR status transitions) and version bumps. This means:

- No way to communicate the impact of a release to consumers
- No audit trail connecting a version bump to the feature/decision that prompted it
- Version strings (0.1.0 everywhere) carry no information

The question: should FDR lifecycle transitions drive version bumps (e.g.,
FDR→accepted = minor bump), or should versioning remain decoupled with changelog
entries linking back to FDRs?

## Interface

<!-- To be filled when solution is selected (status → proposed) -->

## Examples

<!-- To be filled when solution is selected (status → proposed) -->
