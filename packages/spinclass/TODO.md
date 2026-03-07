# TODO

- [ ] Add unit tests for `shop.Attach` using a mock `Executor` — assert that
  the correct `dir`, `key`, and `command` args are passed through (e.g. zmx
  session key format, nix develop wrapping, claude arg forwarding)
- [ ] fix rebase / merge issues caused by `sc merge` — rebase fails with merge conflict when TODO.md (or other frequently-edited files) diverge between worktree and master
