// This file implements LSP-style transport using Content-Length headers.
// For MCP servers using newline-delimited JSON, see the transport package.
package jsonrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

type Stream struct {
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex
}

func NewStream(r io.Reader, w io.Writer) *Stream {
	return &Stream{
		reader: bufio.NewReader(r),
		writer: w,
	}
}

func (s *Stream) Read() (*Message, error) {
	contentLength := -1

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("reading header line: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header line: %s", line)
		}

		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if strings.EqualFold(name, "Content-Length") {
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("parsing Content-Length: %w", err)
			}
			contentLength = n
		}
	}

	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(s.reader, body); err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	var msg Message
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("parsing message: %w", err)
	}

	return &msg, nil
}

func (s *Stream) Write(msg *Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := io.WriteString(s.writer, header); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	if _, err := s.writer.Write(body); err != nil {
		return fmt.Errorf("writing body: %w", err)
	}

	return nil
}
