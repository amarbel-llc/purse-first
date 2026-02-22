
default: build test

build: build-nix

build-nix:
  nix build

test: test-bats

test-bats:
  ./result/bin/bats --no-sandbox zz-tests_bats/bats_wrapper.bats

check:
    nix develop --command shellcheck lib/*/load.bash lib/*/src/*.bash

fmt:
  nix develop --command shfmt -w -i 2 -ci lib/*/load.bash lib/*/src/*.bash

clean:
  rm -rf result

update: update-nix

update-nix:
  nix flake update
