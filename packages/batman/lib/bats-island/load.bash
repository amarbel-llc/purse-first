# bats-island - Test isolation helpers for BATS

# shellcheck disable=1090
source "$(dirname "${BASH_SOURCE[0]}")/src/set_xdg.bash"
source "$(dirname "${BASH_SOURCE[0]}")/src/setup_test_home.bash"
source "$(dirname "${BASH_SOURCE[0]}")/src/setup_test_repo.bash"
source "$(dirname "${BASH_SOURCE[0]}")/src/chflags_nouchg.bash"
source "$(dirname "${BASH_SOURCE[0]}")/src/teardown_test_home.bash"
