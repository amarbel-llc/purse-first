package ohio

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
)

type erroringWriter struct{ err error }

func (e erroringWriter) Write(_ []byte) (int, error) { return 0, e.err }

func TestPipedWriterToCopiesProducerOutputToConsumer(t *testing.T) {
	pw := MakePipedWriterTo()

	var sink bytes.Buffer

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := pw.WriteTo(&sink); err != nil {
			t.Errorf("WriteTo: %v", err)
		}
	}()

	for _, chunk := range []string{"hello ", "piped ", "world"} {
		if _, err := io.WriteString(pw, chunk); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}

	if err := pw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	wg.Wait()

	if got, want := sink.String(), "hello piped world"; got != want {
		t.Errorf("sink = %q, want %q", got, want)
	}
}

func TestPipedWriterToConsumerErrorUnblocksProducer(t *testing.T) {
	pw := MakePipedWriterTo()

	sinkErr := errors.New("sink broken")
	sink := erroringWriter{err: sinkErr}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := pw.WriteTo(sink)
		if !errors.Is(err, sinkErr) {
			t.Errorf("WriteTo err = %v, want %v", err, sinkErr)
		}
	}()

	for {
		_, err := io.WriteString(pw, "data")
		if err != nil {
			if !errors.Is(err, sinkErr) {
				t.Errorf("Write err = %v, want %v", err, sinkErr)
			}
			break
		}
	}

	if err := pw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	wg.Wait()
}

func TestPipedWriterToEmptyProducerWritesNothing(t *testing.T) {
	pw := MakePipedWriterTo()

	var sink bytes.Buffer

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		n, err := pw.WriteTo(&sink)
		if err != nil {
			t.Errorf("WriteTo: %v", err)
		}
		if n != 0 {
			t.Errorf("WriteTo n = %d, want 0", n)
		}
	}()

	if err := pw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	wg.Wait()

	if sink.Len() != 0 {
		t.Errorf("sink = %q, want empty", sink.String())
	}
}
