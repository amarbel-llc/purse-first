package ohio

import (
	"io"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/0/interfaces"
	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/pool"
	"github.com/amarbel-llc/purse-first/libs/dewey/internal/bravo/errors"
)

type PipedReader interface {
	Close() (n int64, err error)
	io.Writer
}

type readFromDone struct {
	n   int64
	err error
}

type pipedReaderFrom struct {
	*io.PipeWriter
	ch chan readFromDone
}

func MakePipedReaderFrom(r io.ReaderFrom) PipedReader {
	var p pipedReaderFrom

	var pr *io.PipeReader
	pr, p.PipeWriter = io.Pipe()
	p.ch = make(chan readFromDone)

	go func() {
		var msg readFromDone

		if msg.n, msg.err = r.ReadFrom(pr); msg.err != nil {
			if !errors.IsEOF(msg.err) {
				pr.CloseWithError(msg.err)
			}
		}

		p.ch <- msg
	}()

	return p
}

func (p pipedReaderFrom) Close() (n int64, err error) {
	if p.PipeWriter == nil {
		return n, err
	}

	p.PipeWriter.Close()
	out := <-p.ch
	n = out.n
	err = out.err

	return n, err
}

type pipedDecoderFrom struct {
	*io.PipeWriter
	ch chan readFromDone
}

func MakePipedDecoder[O any](
	object O,
	decoder interfaces.DecoderFromBufferedReader[O],
) PipedReader {
	var p pipedDecoderFrom

	var pr *io.PipeReader
	pr, p.PipeWriter = io.Pipe()
	p.ch = make(chan readFromDone)

	go func() {
		var msg readFromDone

		bufferedReader, repoolBufferedReader := pool.GetBufferedReader(pr)
		defer repoolBufferedReader()

		if msg.n, msg.err = decoder.DecodeFrom(
			object,
			bufferedReader,
		); msg.err != nil {
			if !errors.IsEOF(msg.err) {
				pr.CloseWithError(msg.err)
			}
		}

		p.ch <- msg
	}()

	return p
}

func (p pipedDecoderFrom) Close() (n int64, err error) {
	if p.PipeWriter == nil {
		return n, err
	}

	p.PipeWriter.Close()
	out := <-p.ch
	n = out.n
	err = out.err

	return n, err
}

// PipedWriter is the symmetric mirror of PipedReader. Producers write to
// the pipe via Write; consumers drain it once via WriteTo(out). Close on
// the producer side signals EOF to a blocked WriteTo, which then returns.
//
// Unlike PipedReader, PipedWriter does not own a consuming goroutine —
// the WriteTo call is the consumer. WriteTo must therefore be called for
// the pipe to drain. If no consumer ever calls WriteTo, every Write will
// block: io.Pipe is synchronous and has no internal buffer.
type PipedWriter interface {
	io.WriterTo
	io.WriteCloser
}

type pipedWriterTo struct {
	*io.PipeWriter
	pr *io.PipeReader
}

func MakePipedWriterTo() PipedWriter {
	pr, pw := io.Pipe()
	return pipedWriterTo{PipeWriter: pw, pr: pr}
}

func (p pipedWriterTo) WriteTo(out io.Writer) (int64, error) {
	n, err := io.Copy(out, p.pr)
	if err != nil {
		p.pr.CloseWithError(err)
	}
	return n, err
}
