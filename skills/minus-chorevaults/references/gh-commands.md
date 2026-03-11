# gh CLI Command Reference

Exact commands used by the upstream-prep skill. This reference prevents hallucinating flags or output shapes.

## Repository Discovery

### Identify upstream for a fork

```sh
gh repo view --json parent,defaultBranchRef
```

Output when repo is a fork:
```json
{
  "defaultBranchRef": {
    "name": "main"
  },
  "parent": {
    "name": "repo-name",
    "owner": {
      "login": "upstream-owner"
    }
  }
}
```

Output when repo is NOT a fork:
```json
{
  "defaultBranchRef": {
    "name": "main"
  },
  "parent": null
}
```

### Get repo merge settings

```sh
gh api repos/{owner}/{repo} --jq '{
  allow_squash_merge,
  allow_merge_commit,
  allow_rebase_merge,
  delete_branch_on_merge
}'
```

## Fetching Upstream Files

### CONTRIBUTING.md

```sh
gh api repos/{owner}/{repo}/contents/CONTRIBUTING.md --jq '.content' | base64 -d
```

Returns 404 if file doesn't exist. Handle gracefully.

### PR Template

```sh
gh api repos/{owner}/{repo}/contents/.github/PULL_REQUEST_TEMPLATE.md --jq '.content' | base64 -d
```

Also check for multiple templates:
```sh
gh api repos/{owner}/{repo}/contents/.github/PULL_REQUEST_TEMPLATE --jq '.[].name'
```

### EditorConfig

```sh
gh api repos/{owner}/{repo}/contents/.editorconfig --jq '.content' | base64 -d
```

## Issue and PR Discovery

### Search for related issues in upstream

```sh
gh issue list --repo {owner}/{repo} --search "<query>" --json number,title,url,state --limit 10
```

Output:
```json
[
  {
    "number": 42,
    "title": "Support feature X",
    "url": "https://github.com/owner/repo/issues/42",
    "state": "OPEN"
  }
]
```

### Search across GitHub for matching issues

```sh
gh search issues "<query>" --repo {owner}/{repo} --json number,title,url --limit 10
```

### Check for existing PRs from user to upstream

```sh
gh pr list --repo {owner}/{repo} --author {user} --json number,title,url,state,headRefName
```

Output:
```json
[
  {
    "number": 99,
    "title": "Add feature X",
    "url": "https://github.com/owner/repo/pull/99",
    "state": "OPEN",
    "headRefName": "friedenberg/feature-x"
  }
]
```

### List open PRs to find branch naming conventions

```sh
gh pr list --repo {owner}/{repo} --json headRefName --limit 20
```

## PR Creation

### Create a new PR against upstream

```sh
gh pr create \
  --repo {upstream-owner}/{repo} \
  --title "PR title here" \
  --body "$(cat <<'EOF'
PR body here
EOF
)" \
  --head {fork-owner}:{branch} \
  --base {upstream-default-branch}
```

Important flags:
- `--repo` — the upstream repository (not the fork)
- `--head` — must be `{fork-owner}:{branch}` format for cross-repo PRs
- `--base` — the upstream branch to merge into (usually `main` or `master`)

### Update an existing PR

```sh
gh pr edit {number} \
  --repo {upstream-owner}/{repo} \
  --title "Updated title" \
  --body "$(cat <<'EOF'
Updated body
EOF
)"
```

## Git Operations

### Get the merge base between fork and upstream

```sh
git merge-base upstream/{default-branch} HEAD
```

Requires the upstream remote to be configured:
```sh
git remote add upstream https://github.com/{owner}/{repo}.git
git fetch upstream {default-branch}
```

### View fork's divergent commits

```sh
git log --oneline upstream/{default-branch}..HEAD
```

### Check for merge conflicts

```sh
git diff upstream/{default-branch}...HEAD --name-only
```

### Interactive rebase for commit cleanup

```sh
git rebase -i upstream/{default-branch}
```

Note: this is interactive and cannot be run by the skill directly. Instead, provide rebase instructions to the user or use `git rebase` with `--exec` for automated fixups.
