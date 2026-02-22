package executor

type Executor interface {
	Attach(dir string, key string, command []string) error
	Detach() error
}
