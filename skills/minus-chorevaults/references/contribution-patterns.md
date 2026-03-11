# Common Upstream Contribution Patterns

Patterns the upstream-prep skill should detect and adapt to when preparing fork work for submission.

## Commit Message Conventions

### Conventional Commits

Format: `<type>(<scope>): <description>`

Common types:
- `feat:` — new feature
- `fix:` — bug fix
- `chore:` — maintenance, dependencies
- `docs:` — documentation only
- `refactor:` — code change that neither fixes a bug nor adds a feature
- `test:` — adding or correcting tests
- `ci:` — CI/CD changes
- `perf:` — performance improvement

Detection: look for this pattern in recent upstream commits via `git log --oneline`.

### DCO Sign-off

Some projects require Developer Certificate of Origin sign-off on every commit:

```
Signed-off-by: Full Name <email@example.com>
```

Detection: check CONTRIBUTING.md for "DCO" or "sign-off", or look for `Signed-off-by:` trailers in upstream commits.

Add with: `git commit -s` or `git commit --amend -s` for existing commits.

### Commit Scope

Some projects require commits scoped to a module or subsystem:

```
feat(parser): add support for nested expressions
fix(auth): handle expired tokens gracefully
```

Detection: check if >50% of recent upstream commits use parenthesized scopes.

## CLA Requirements

### CLA-assistant

- GitHub App that comments on PRs from first-time contributors
- Requires signing via web form
- Detection: check for `.clabot` file or CLA mentions in CONTRIBUTING.md

### Google CLA

- Required for all Google open-source projects
- Individual and corporate CLAs
- Detection: CONTRIBUTING.md mentions "Contributor License Agreement" or links to `cla.developers.google.com`

### Apache ICLA

- Required for Apache Foundation projects
- Detection: CONTRIBUTING.md references "ICLA" or "Individual Contributor License Agreement"

When CLA is detected, warn the user before opening the PR.

## PR Templates

### Checkbox-style Templates

Many projects use templates with required checkboxes:

```markdown
- [ ] I have read the contributing guidelines
- [ ] My code follows the project's code style
- [ ] I have added tests for my changes
- [ ] All new and existing tests pass
- [ ] I have updated documentation as needed
```

Detection: fetch `.github/PULL_REQUEST_TEMPLATE.md` or `.github/PULL_REQUEST_TEMPLATE/` directory from upstream.

When a template exists, fill it in rather than using the fallback template.

### Multiple Templates

Some repos have multiple PR templates in `.github/PULL_REQUEST_TEMPLATE/`:
- `bug_fix.md`
- `feature.md`
- `documentation.md`

Select the most appropriate template based on the nature of the changes.

## Branch Naming Conventions

Common patterns to detect from upstream's existing branches and CONTRIBUTING.md:

| Pattern | Example |
|---------|---------|
| `feature/<name>` | `feature/add-auth` |
| `fix/<name>` | `fix/null-pointer` |
| `<user>/<name>` | `friedenberg/add-auth` |
| `<issue>-<name>` | `123-add-auth` |
| `<type>/<issue>-<name>` | `feat/123-add-auth` |

Detection: examine open PR head branch names via `gh pr list --json headRefName`.

## Merge Strategy Expectations

### Squash Merge

- Single commit per PR on the target branch
- Commit message often matches PR title
- Detection: check repo settings via `gh api repos/{owner}/{repo}` for `allow_squash_merge`, `allow_merge_commit`, `allow_rebase_merge`
- When squash-merge is the norm, individual commit quality matters less — focus on the PR description

### Rebase Merge

- Each commit lands individually on the target branch
- Every commit must be clean and pass CI independently
- Detection: same API check; also look for CONTRIBUTING.md guidance on "rebase" or "clean history"
- When rebase is expected, ensure each commit is atomic and well-described

### Merge Commit

- A merge commit is created
- Less strict about individual commit quality
- Detection: look for merge commits in upstream history

## Changelog Requirements

Some projects require changelog entries with PRs:

- `CHANGELOG.md` — manual entries, often under an "Unreleased" section
- `changes/` or `changelog.d/` directory — towncrier-style fragments
- `.changes/` — per-PR changelog files

Detection: check for these files/directories in the repo root. Check CONTRIBUTING.md for "changelog" mentions.

## Test Requirements

Common expectations:

- "All tests must pass" — standard CI gate
- "Add tests for new features" — check CONTRIBUTING.md
- "Maintain or improve code coverage" — check for coverage tools in CI config
- Specific test frameworks — detect from the project's test infrastructure

Detection: read CI config (`.github/workflows/`, `.travis.yml`, etc.) and CONTRIBUTING.md.

## Code Style

### Formatters and Linters

| Language | Formatter | Linter |
|----------|-----------|--------|
| Go | `gofmt`, `goimports`, `gofumpt` | `golangci-lint` |
| Rust | `rustfmt` | `clippy` |
| Python | `black`, `ruff` | `ruff`, `flake8`, `mypy` |
| JavaScript/TypeScript | `prettier` | `eslint` |
| Nix | `nixfmt`, `alejandra` | `statix` |
| Shell | `shfmt` | `shellcheck` |

Detection: check for config files (`.editorconfig`, `rustfmt.toml`, `.prettierrc`, `.golangci.yml`, `pyproject.toml`) and CI steps that run formatters/linters.

### EditorConfig

`.editorconfig` specifies indent style, indent size, end-of-line, and charset. When present, verify fork changes comply before submitting.
