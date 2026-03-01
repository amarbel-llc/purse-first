set_xdg() {
  if [[ -z "${1:-}" ]]; then
    echo "set_xdg: base directory argument required" >&2
    return 1
  fi

  local loc
  loc="$(realpath "$1" 2>/dev/null)"

  if [[ -z "$loc" ]]; then
    echo "set_xdg: realpath failed for '$1'" >&2
    return 1
  fi

  export XDG_DATA_HOME="$loc/.xdg/data"
  export XDG_CONFIG_HOME="$loc/.xdg/config"
  export XDG_STATE_HOME="$loc/.xdg/state"
  export XDG_CACHE_HOME="$loc/.xdg/cache"
  export XDG_RUNTIME_HOME="$loc/.xdg/runtime"
  mkdir -p "$XDG_DATA_HOME" "$XDG_CONFIG_HOME" "$XDG_STATE_HOME" \
    "$XDG_CACHE_HOME" "$XDG_RUNTIME_HOME"
}
