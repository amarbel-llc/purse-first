#!/usr/bin/env bash
set -e

real_home="$HOME"
tmp_home="$(mktemp -d /tmp/purse-first-lifecycle-XXXXXX)"

srt_config="$(mktemp)"
trap 'rm -f "$srt_config"' EXIT

cat >"$srt_config" <<SETTINGS
{
  "filesystem": {
    "denyRead": [
      "$real_home/.claude",
      "$real_home/.ssh",
      "$real_home/.aws",
      "$real_home/.gnupg",
      "$real_home/.config",
      "$real_home/.password-store",
      "$real_home/.kube"
    ],
    "denyWrite": [],
    "allowWrite": [
      "/tmp"
    ]
  },
  "network": {
    "allowedDomains": [],
    "deniedDomains": []
  }
}
SETTINGS

HOME="$tmp_home" \
  PURSE_FIRST_REAL_HOME="$real_home" \
  exec sandcastle \
    --shell bash \
    --config "$srt_config" \
    "$@"
