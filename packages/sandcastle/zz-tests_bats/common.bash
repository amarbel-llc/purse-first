bats_load_library bats-support
bats_load_library bats-assert
bats_load_library bats-emo

require_bin SANDCASTLE_BIN sandcastle
SANDCASTLE_BIN="${SANDCASTLE_BIN:-sandcastle}"
