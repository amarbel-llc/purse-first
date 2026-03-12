#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  purse_first="$(purse_first_bin)"
  OUTPUT_DIR="${BATS_TEST_TMPDIR}/brew-output"

  # Create fake binary.
  BIN_DIR="${BATS_TEST_TMPDIR}/bins"
  mkdir -p "$BIN_DIR"
  printf '#!/bin/sh\necho hello' > "$BIN_DIR/my-tool"
  chmod +x "$BIN_DIR/my-tool"

  # Create share directories.
  SHARE_BASE="${BATS_TEST_TMPDIR}/shares"

  TOOL_SHARE="$SHARE_BASE/my-tool"
  mkdir -p "$TOOL_SHARE/.claude-plugin"
  cat > "$TOOL_SHARE/.claude-plugin/plugin.json" <<'EOF'
{"name":"my-tool","mcpServers":{"my-tool":{"type":"stdio","command":"my-tool"}}}
EOF

  SKILL_SHARE="$SHARE_BASE/my-skills"
  mkdir -p "$SKILL_SHARE/.claude-plugin"
  mkdir -p "$SKILL_SHARE/skills/test-skill"
  echo '{"name":"my-skills"}' > "$SKILL_SHARE/.claude-plugin/plugin.json"
  printf -- '---\nname: test\n---\nContent' > "$SKILL_SHARE/skills/test-skill/SKILL.md"

  # Write config.
  CONFIG_PATH="${BATS_TEST_TMPDIR}/brew-config.json"
  cat > "$CONFIG_PATH" <<CONF
{
  "name": "test-marketplace",
  "description": "Test marketplace",
  "owner": {"name": "tester"},
  "releaseRepo": "org/tap",
  "tapName": "org/tap",
  "license": "MIT",
  "packages": {
    "my-tool": {
      "description": "A tool",
      "version": "1.0.0",
      "binary": true,
      "platforms": {"darwin-arm64": "$BIN_DIR/my-tool"},
      "share": "$TOOL_SHARE",
      "brewDeps": ["gh"]
    },
    "my-skills": {
      "description": "Skills package",
      "version": "0.1.0",
      "binary": false,
      "share": "$SKILL_SHARE",
      "brewDeps": []
    }
  }
}
CONF
}

@test "package brew generates Formula directory" {
  run "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  assert_success
  [[ -d "$OUTPUT_DIR/Formula" ]]
}

@test "package brew generates per-package formulas" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/Formula/my-tool.rb" ]]
  [[ -f "$OUTPUT_DIR/Formula/my-skills.rb" ]]
}

@test "package brew generates meta formula" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/Formula/test-marketplace.rb" ]]
}

@test "package brew generates tarballs" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/tarballs/my-tool-1.0.0-darwin-arm64.tar.gz" ]]
  [[ -f "$OUTPUT_DIR/tarballs/my-skills-0.1.0.tar.gz" ]]
  [[ -f "$OUTPUT_DIR/tarballs/test-marketplace-0.1.0.tar.gz" ]]
}

@test "package brew generates marketplace.json" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/.claude-plugin/marketplace.json" ]]
}

@test "package brew generates README" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/README.md" ]]
}

@test "binary formula has bin.install" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'bin.install' "$OUTPUT_DIR/Formula/my-tool.rb"
}

@test "skill-only formula lacks bin.install" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  ! grep -q 'bin.install' "$OUTPUT_DIR/Formula/my-skills.rb"
}

@test "formula has real sha256 hashes" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  ! grep -q 'SHA256_PLACEHOLDER' "$OUTPUT_DIR/Formula/my-tool.rb"
  grep -qE 'sha256 "[a-f0-9]{64}"' "$OUTPUT_DIR/Formula/my-tool.rb"
}

@test "meta formula has depends_on for all packages" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'depends_on "my-tool"' "$OUTPUT_DIR/Formula/test-marketplace.rb"
  grep -q 'depends_on "my-skills"' "$OUTPUT_DIR/Formula/test-marketplace.rb"
}

@test "meta formula has post_install by default" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'system "purse-first", "install"' "$OUTPUT_DIR/Formula/test-marketplace.rb"
}

@test "meta formula omits post_install with --no-auto-install" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR" --no-auto-install
  ! grep -q 'post_install' "$OUTPUT_DIR/Formula/test-marketplace.rb"
}

@test "binary formula has brew dependency" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'depends_on "gh"' "$OUTPUT_DIR/Formula/my-tool.rb"
}

@test "formula class names are PascalCase" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'class MyTool < Formula' "$OUTPUT_DIR/Formula/my-tool.rb"
  grep -q 'class MySkills < Formula' "$OUTPUT_DIR/Formula/my-skills.rb"
  grep -q 'class TestMarketplace < Formula' "$OUTPUT_DIR/Formula/test-marketplace.rb"
}

# --- Private repo tests ---

setup_private_config() {
  PRIVATE_OUTPUT_DIR="${BATS_TEST_TMPDIR}/brew-private-output"
  PRIVATE_CONFIG_PATH="${BATS_TEST_TMPDIR}/brew-config-private.json"
  cat > "$PRIVATE_CONFIG_PATH" <<CONF
{
  "name": "test-marketplace",
  "description": "Test marketplace",
  "owner": {"name": "tester"},
  "releaseRepo": "org/tap",
  "tapName": "org/tap",
  "license": "MIT",
  "private": true,
  "packages": {
    "my-tool": {
      "description": "A tool",
      "version": "1.0.0",
      "binary": true,
      "platforms": {"darwin-arm64": "$BIN_DIR/my-tool"},
      "share": "$TOOL_SHARE",
      "brewDeps": []
    }
  }
}
CONF
}

@test "private config generates download strategy file" {
  setup_private_config
  "$purse_first" package brew --config "$PRIVATE_CONFIG_PATH" --output "$PRIVATE_OUTPUT_DIR"
  [[ -f "$PRIVATE_OUTPUT_DIR/lib/custom_download_strategy.rb" ]]
}

@test "private config formula has require_relative" {
  setup_private_config
  "$purse_first" package brew --config "$PRIVATE_CONFIG_PATH" --output "$PRIVATE_OUTPUT_DIR"
  grep -q 'require_relative "../lib/custom_download_strategy"' "$PRIVATE_OUTPUT_DIR/Formula/my-tool.rb"
}

@test "private config formula has using: directive" {
  setup_private_config
  "$purse_first" package brew --config "$PRIVATE_CONFIG_PATH" --output "$PRIVATE_OUTPUT_DIR"
  grep -q 'using: GitHubPrivateRepositoryReleaseDownloadStrategy' "$PRIVATE_OUTPUT_DIR/Formula/my-tool.rb"
}

@test "private config meta-formula has require_relative" {
  setup_private_config
  "$purse_first" package brew --config "$PRIVATE_CONFIG_PATH" --output "$PRIVATE_OUTPUT_DIR"
  grep -q 'require_relative "../lib/custom_download_strategy"' "$PRIVATE_OUTPUT_DIR/Formula/test-marketplace.rb"
}

@test "private config meta-formula has using: directive" {
  setup_private_config
  "$purse_first" package brew --config "$PRIVATE_CONFIG_PATH" --output "$PRIVATE_OUTPUT_DIR"
  grep -q 'using: GitHubPrivateRepositoryReleaseDownloadStrategy' "$PRIVATE_OUTPUT_DIR/Formula/test-marketplace.rb"
}

@test "non-private config omits download strategy file" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ ! -f "$OUTPUT_DIR/lib/custom_download_strategy.rb" ]]
}

@test "non-private config formula omits require_relative" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  ! grep -q 'require_relative' "$OUTPUT_DIR/Formula/my-tool.rb"
}

@test "non-private config formula omits using: directive" {
  "$purse_first" package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  ! grep -q 'using:' "$OUTPUT_DIR/Formula/my-tool.rb"
}
