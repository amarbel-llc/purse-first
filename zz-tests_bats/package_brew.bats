#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
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
  run purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  assert_success
  [[ -d "$OUTPUT_DIR/Formula" ]]
}

@test "package brew generates per-package formulas" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/Formula/my-tool.rb" ]]
  [[ -f "$OUTPUT_DIR/Formula/my-skills.rb" ]]
}

@test "package brew generates meta formula" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/Formula/test-marketplace.rb" ]]
}

@test "package brew generates tarballs" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/tarballs/my-tool-1.0.0-darwin-arm64.tar.gz" ]]
  [[ -f "$OUTPUT_DIR/tarballs/my-skills-0.1.0.tar.gz" ]]
  [[ -f "$OUTPUT_DIR/tarballs/test-marketplace-0.1.0.tar.gz" ]]
}

@test "package brew generates marketplace.json" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/.claude-plugin/marketplace.json" ]]
}

@test "package brew generates README" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/README.md" ]]
}

@test "binary formula has bin.install" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'bin.install' "$OUTPUT_DIR/Formula/my-tool.rb"
}

@test "skill-only formula lacks bin.install" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  ! grep -q 'bin.install' "$OUTPUT_DIR/Formula/my-skills.rb"
}

@test "formula has real sha256 hashes" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  ! grep -q 'SHA256_PLACEHOLDER' "$OUTPUT_DIR/Formula/my-tool.rb"
  grep -qE 'sha256 "[a-f0-9]{64}"' "$OUTPUT_DIR/Formula/my-tool.rb"
}

@test "meta formula has depends_on for all packages" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'depends_on "my-tool"' "$OUTPUT_DIR/Formula/test-marketplace.rb"
  grep -q 'depends_on "my-skills"' "$OUTPUT_DIR/Formula/test-marketplace.rb"
}

@test "meta formula has post_install by default" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'system "purse-first", "install"' "$OUTPUT_DIR/Formula/test-marketplace.rb"
}

@test "meta formula omits post_install with --no-auto-install" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR" --no-auto-install
  ! grep -q 'post_install' "$OUTPUT_DIR/Formula/test-marketplace.rb"
}

@test "binary formula has brew dependency" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'depends_on "gh"' "$OUTPUT_DIR/Formula/my-tool.rb"
}

@test "formula class names are PascalCase" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'class MyTool < Formula' "$OUTPUT_DIR/Formula/my-tool.rb"
  grep -q 'class MySkills < Formula' "$OUTPUT_DIR/Formula/my-skills.rb"
  grep -q 'class TestMarketplace < Formula' "$OUTPUT_DIR/Formula/test-marketplace.rb"
}
