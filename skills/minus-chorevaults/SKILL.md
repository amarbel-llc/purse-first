---
name: Upstream Prep
description: This skill should be used when the user asks to "prepare for upstream", "upstream PR", "submit upstream", "integrate fork work", "open PR against upstream", "send changes upstream", "contribute back", "create upstream pull request", or mentions upstream integration, fork-to-upstream workflow, or cross-repo PR creation.
argument-hint: "[branch-or-path]"
disable-model-invocation: true
version: 0.1.0
---

# Upstream Prep

This skill prepares fork work for upstream integration. It discovers the upstream repository, analyzes contribution guidelines and commit history, checks for related issues and PRs, and produces a clean, well-structured pull request.

The user works on personal forks where only `origin` exists. Upstream is discovered via GitHub fork metadata.

## Mode Selection

Before starting, ask the user which mode to use:

- **Interactive** (default) — explain each step and confirm before executing. Best for first-time contributions or unfamiliar upstreams.
- **Automated** — run all phases without pausing, only confirm at PR creation. Best for repeat contributions to known upstreams.

If the user provided no preference, default to interactive.

## Phase 1: Discovery

Gather all context about the upstream repo and the fork's divergent work.

### 1.1 Detect Current State

```sh
# Current repo and branch
git rev-parse --abbrev-ref HEAD
git remote -v

# Fork's upstream parent
gh repo view --json parent,defaultBranchRef
```

If `parent` is null, the repo is not a fork. Ask the user to provide the upstream repo in `owner/repo` format.

### 1.2 Fetch Upstream Metadata

Add upstream remote temporarily (if not already present) and fetch:

```sh
git remote add upstream https://github.com/{owner}/{repo}.git
git fetch upstream {default-branch}
```

### 1.3 Retrieve Contribution Guidelines

Fetch these files from upstream (handle 404s gracefully — not all repos have them):

- `CONTRIBUTING.md` — contribution rules, CLA requirements, commit conventions
- `.github/PULL_REQUEST_TEMPLATE.md` — PR template to fill in
- `.github/PULL_REQUEST_TEMPLATE/` — multiple PR templates (pick the best match)
- `.editorconfig` — formatting rules
- Linter/formatter configs: `.golangci.yml`, `rustfmt.toml`, `.prettierrc`, `pyproject.toml`, etc.

Use `gh api repos/{owner}/{repo}/contents/{path}` to fetch each. See `references/gh-commands.md` for exact commands and output shapes.

### 1.4 Find Related Issues and PRs

```sh
# Search upstream issues matching the branch topic
gh issue list --repo {owner}/{repo} --search "<branch-topic>" --json number,title,url,state --limit 10

# Search more broadly
gh search issues "<branch-topic>" --repo {owner}/{repo} --json number,title,url --limit 10

# Check if user already has open PRs to upstream
gh pr list --repo {owner}/{repo} --author {user} --json number,title,url,state,headRefName
```

If an existing open PR from the user covers similar work, note it — Phase 4 can update it instead of creating a new one.

## Phase 2: Analysis

Analyze the fork's changes against upstream context.

### 2.1 Summarize Divergent Work

```sh
git log --oneline upstream/{default-branch}..HEAD
git diff --stat upstream/{default-branch}..HEAD
```

Summarize:
- Number of commits and files changed
- Nature of changes (new feature, bug fix, refactor, docs, etc.)
- Which subsystems or modules are affected

### 2.2 Check Style Compliance

Using configs discovered in Phase 1:
- If `.editorconfig` exists, verify indent style/size in changed files
- If language-specific formatter config exists, note whether changes comply
- If linting config exists, note any obvious violations

Do NOT auto-run formatters yet — that's Phase 3.

### 2.3 Identify Potential Conflicts

```sh
git merge-base upstream/{default-branch} HEAD
git diff upstream/{default-branch}...HEAD --name-only
```

Check if upstream has changed the same files since the merge base. If so, flag potential conflicts and recommend a rebase strategy.

### 2.4 Flag Duplicates and Related Work

From the issue/PR search results in Phase 1:
- Flag if an existing upstream PR covers similar work (link it)
- Flag if there's an issue that this work addresses (suggest "Fixes #N")
- Flag if someone else is actively working on the same area

In interactive mode, present findings and ask how to proceed.

## Phase 3: Preparation

Prepare the changes for submission.

### 3.1 Commit Strategy

Recommend one of these based on upstream conventions (see `references/contribution-patterns.md`):

- **Squash** — combine all commits into one logical commit. Best when upstream uses squash-merge or when the fork has messy WIP commits.
- **Rebase** — rebase onto upstream HEAD, keeping individual commits. Best when upstream uses rebase-merge and each commit is meaningful.
- **Keep as-is** — leave commits untouched. Only when they're already clean and upstream has no strong preference.

Check upstream merge settings:
```sh
gh api repos/{owner}/{repo} --jq '{allow_squash_merge, allow_merge_commit, allow_rebase_merge}'
```

### 3.2 Commit Message Conventions

Detect upstream's commit message style from recent history:
```sh
git log upstream/{default-branch} --oneline -20
```

Common patterns to detect and apply (see `references/contribution-patterns.md`):
- Conventional Commits (`feat:`, `fix:`, `chore:`)
- DCO sign-off (`Signed-off-by:`)
- Scoped commits (`feat(module):`)
- Issue references in commit messages

If rewriting commit messages, confirm with the user first.

### 3.3 Run Formatters and Linters

Based on configs found in Phase 1, run any applicable formatters on changed files:
- Go: `gofmt`, `goimports`, `gofumpt`
- Rust: `cargo fmt`
- Python: `black`, `ruff format`
- JavaScript/TypeScript: `prettier`
- Nix: `nixfmt`
- Shell: `shfmt`

Only run formatters that the upstream project actually uses (detected from configs and CI).

### 3.4 Draft PR Description

Build the PR description using this priority:
1. Upstream's PR template (from `.github/PULL_REQUEST_TEMPLATE.md`) — fill it in
2. Fallback template from `examples/pr-description.md`

Include:
- Summary of what the changes do and why
- "Fixes #N" if a matching issue was found
- Test plan or testing notes
- Any breaking changes or migration notes

Present the draft to the user for review before proceeding.

## Phase 4: Execution

Submit the prepared work to upstream.

### 4.1 Branch Naming

Check upstream's branch naming convention from open PRs:
```sh
gh pr list --repo {owner}/{repo} --json headRefName --limit 20
```

If a pattern is clear (e.g., `feature/`, `fix/`, `user/`), rename the branch to match. Otherwise keep the current branch name.

### 4.2 Push to Fork

Force-push the prepared branch to origin:

```sh
git push --force-with-lease origin {branch}
```

**Always confirm with the user before force-pushing.** Explain what will change.

### 4.3 Create or Update PR

If an existing open PR from the user was found in Phase 1:
```sh
gh pr edit {number} \
  --repo {upstream-owner}/{repo} \
  --title "PR title" \
  --body "$(cat <<'EOF'
PR body
EOF
)"
```

Otherwise create a new PR:
```sh
gh pr create \
  --repo {upstream-owner}/{repo} \
  --title "PR title" \
  --body "$(cat <<'EOF'
PR body
EOF
)" \
  --head {fork-owner}:{branch} \
  --base {upstream-default-branch}
```

See `references/gh-commands.md` for exact flag usage.

### 4.4 Report

After PR creation/update, report:
- PR URL
- Summary of what was submitted
- Any follow-up actions needed (CLA signing, CI checks to watch, reviewer to ping)

## Edge Cases

### Repo is Not a Fork

When `gh repo view --json parent` returns `parent: null`:
- Ask the user for the upstream repo in `owner/repo` format
- Continue with that as the upstream target

### CLA Requirements Detected

When CONTRIBUTING.md mentions CLA, DCO, or contributor agreements:
- Warn the user before opening the PR
- Link to the CLA signing page if identifiable
- For DCO, offer to add `Signed-off-by` trailers to all commits

### Multiple Branches to Upstream

If the user has work across multiple branches that should go upstream:
- Ask which branch to prepare first
- Note that separate PRs are usually preferred over one large PR

### Upstream Has Diverged Significantly

When the merge base is far behind upstream HEAD:
- Recommend rebasing onto upstream HEAD before submitting
- Warn about potential conflicts
- Offer to walk through conflict resolution in interactive mode

### Existing Open PR from User

When the user already has an open PR to upstream:
- Show the existing PR details
- Ask whether to update it or create a new one
- If updating, use `gh pr edit` instead of `gh pr create`

### No Origin Remote

When `git remote -v` shows no `origin`:
- Ask the user for their fork's URL
- Add it as `origin`

## Additional Resources

### Reference Files

For exact CLI commands and expected outputs:
- **`references/gh-commands.md`** — every `gh` command used by this skill with output shapes

For upstream convention detection:
- **`references/contribution-patterns.md`** — commit styles, CLA types, PR templates, branch naming, merge strategies

### Example Files

- **`examples/pr-description.md`** — fallback PR description template when upstream has no PR template
