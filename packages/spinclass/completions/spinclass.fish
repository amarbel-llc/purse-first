complete \
  --command spinclass \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "open" \
  --description "open a worktree shop"

complete \
  --command spinclass \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "attach" \
  --description "attach to a worktree session"

complete \
  --command spinclass \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "status" \
  --description "show status of all repos and worktrees"

complete \
  --command spinclass \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "merge" \
  --description "merge current worktree into main"

complete \
  --command spinclass \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "clean" \
  --description "remove merged worktrees"

complete \
  --command spinclass \
  --no-files \
  --keep-order \
  --condition "__fish_seen_subcommand_from open attach" \
  --arguments "(spinclass completions)"
