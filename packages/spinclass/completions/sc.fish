# Subcommands
complete \
  --command sc \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "open" \
  --description "open a worktree shop"

complete \
  --command sc \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "attach" \
  --description "attach to a worktree session"

complete \
  --command sc \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "create" \
  --description "create a worktree without attaching"

complete \
  --command sc \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "status" \
  --description "show status of all repos and worktrees"

complete \
  --command sc \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "merge" \
  --description "merge current worktree into main"

complete \
  --command sc \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "clean" \
  --description "remove merged worktrees"

complete \
  --command sc \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "pull" \
  --description "pull repos and rebase worktrees"

complete \
  --command sc \
  --no-files \
  --condition __fish_use_subcommand \
  --arguments "perms" \
  --description "manage Claude Code permission tiers"

# Global flags
complete \
  --command sc \
  --no-files \
  --long-option format \
  --require-parameter \
  --arguments "tap table" \
  --description "output format"

# Dynamic target completions for open/attach/create
complete \
  --command sc \
  --no-files \
  --keep-order \
  --condition "__fish_seen_subcommand_from open attach create" \
  --arguments "(sc completions)"

# create flags
complete \
  --command sc \
  --no-files \
  --condition "__fish_seen_subcommand_from create" \
  --short-option v \
  --long-option verbose \
  --description "print sweatfile loading details"

# clean flags
complete \
  --command sc \
  --no-files \
  --condition "__fish_seen_subcommand_from clean" \
  --short-option i \
  --long-option interactive \
  --description "interactively discard changes in dirty merged worktrees"

# pull flags
complete \
  --command sc \
  --no-files \
  --condition "__fish_seen_subcommand_from pull" \
  --short-option d \
  --long-option dirty \
  --description "include dirty repos and worktrees"

# perms subcommands
complete \
  --command sc \
  --no-files \
  --condition "__fish_seen_subcommand_from perms; and not __fish_seen_subcommand_from list edit review" \
  --arguments "list" \
  --description "list permission tier rules"

complete \
  --command sc \
  --no-files \
  --condition "__fish_seen_subcommand_from perms; and not __fish_seen_subcommand_from list edit review" \
  --arguments "edit" \
  --description "edit a permission tier file"

complete \
  --command sc \
  --no-files \
  --condition "__fish_seen_subcommand_from perms; and not __fish_seen_subcommand_from list edit review" \
  --arguments "review" \
  --description "review new permissions from a session"

# perms list/edit flags
complete \
  --command sc \
  --no-files \
  --condition "__fish_seen_subcommand_from list edit" \
  --long-option repo \
  --require-parameter \
  --description "repo name"

complete \
  --command sc \
  --no-files \
  --condition "__fish_seen_subcommand_from edit" \
  --long-option global \
  --description "edit the global tier file"
