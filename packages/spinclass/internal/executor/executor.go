package executor

import (
	tap "github.com/amarbel-llc/purse-first/packages/tap-dancer/go"
)

type Executor interface {
	Attach(dir string, key string, command []string, dryRun bool, tp *tap.TestPoint) error
	Detach() error
}
