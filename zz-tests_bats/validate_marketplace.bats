#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  marketplace_json="$(marketplace_result)"
}

function marketplace_json_exists { # @test
  [[ -f "$marketplace_json" ]]
}

function validate_marketplace_json_passes { # @test
  run claude plugin validate "$marketplace_json"
  assert_success
}

function marketplace_has_required_fields { # @test
  run jq -e '.name' "$marketplace_json"
  assert_success

  run jq -e '.owner.name' "$marketplace_json"
  assert_success

  run jq -e '.plugins | length > 0' "$marketplace_json"
  assert_success
}

function all_plugins_have_github_source { # @test
  run jq -e '[.plugins[] | .source.source] | all(. == "github")' "$marketplace_json"
  assert_success
  assert_output "true"

  run jq -e '[.plugins[] | .source.repo] | all(. != null and . != "")' "$marketplace_json"
  assert_success
  assert_output "true"
}

function plugin_names_match_config { # @test
  run jq -r '[.plugins[].name] | sort | join(",")' "$marketplace_json"
  assert_success
  assert_output "purse-first"
}

function marketplace_has_skills { # @test
  run jq -e '.plugins[0].skills[] | select(endswith("plugin-mcp"))' "$marketplace_json"
  assert_success
}

function description_in_metadata { # @test
  run jq -e '.metadata.description' "$marketplace_json"
  assert_success

  run jq -e '.description // empty' "$marketplace_json"
  assert_failure
}
