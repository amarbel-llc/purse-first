# tap-writer - TAP version 14 output helpers for bash scripts

# shellcheck disable=1090
source "$(dirname "${BASH_SOURCE[0]}")/src/plan.bash"
source "$(dirname "${BASH_SOURCE[0]}")/src/run.bash"
source "$(dirname "${BASH_SOURCE[0]}")/src/skip.bash"
source "$(dirname "${BASH_SOURCE[0]}")/src/comment.bash"
source "$(dirname "${BASH_SOURCE[0]}")/src/bail_out.bash"

_tap_test_num=0
_tap_plan_declared=0

_tap_trailing_plan() {
  if [[ $_tap_plan_declared -eq 0 ]]; then
    echo "1..${_tap_test_num}"
  fi
}

trap _tap_trailing_plan EXIT

echo "TAP version 14"
