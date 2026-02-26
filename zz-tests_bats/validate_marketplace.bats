#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  marketplace_json="$(marketplace_result)"
  purse_first="$(purse_first_bin)"
}

function marketplace_json_exists { # @test
  [[ -f "$marketplace_json" ]]
}

function validate_marketplace_json_passes { # @test
  run claude plugin validate "$marketplace_json"
  assert_success
}

function purse_first_validate_marketplace_json_passes { # @test
  run "$purse_first" validate "$marketplace_json"
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

function all_plugins_have_directory_source { # @test
  run jq -e '[.plugins[] | .source] | all(type == "string" and startswith("./"))' "$marketplace_json"
  assert_success
  assert_output "true"
}

function plugin_names_match_config { # @test
  run jq -r '[.plugins[].name] | sort | join(",")' "$marketplace_json"
  assert_success
  assert_output "bob,chix,get-hubbed,grit,lux,mgp,robin,tap-dancer"
}

function marketplace_has_skills { # @test
  run jq -e '[.plugins[].skills // [] | .[]] | map(select(endswith("creating-packages"))) | length > 0' "$marketplace_json"
  assert_success
}

function description_in_metadata { # @test
  run jq -e '.metadata.description' "$marketplace_json"
  assert_success

  run jq -e '.description // empty' "$marketplace_json"
  assert_failure
}
