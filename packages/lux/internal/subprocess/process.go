package subprocess

import (
	"io"
	"sync"
)

type StderrLogger struct {
	name   string
	writer io.Writer
	mu     sync.Mutex
}

func NewStderrLogger(name string, writer io.Writer) *StderrLogger {
	return &StderrLogger{
		name:   name,
		writer: writer,
	}
}

func (l *StderrLogger) Run(r io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			l.mu.Lock()
			l.writer.Write([]byte("[" + l.name + "] "))
			l.writer.Write(buf[:n])
			l.mu.Unlock()
		}
		if err != nil {
			break
		}
	}
}
