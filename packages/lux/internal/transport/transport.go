package transport

import (
	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

type Transport interface {
	Read() (*jsonrpc.Message, error)
	Write(*jsonrpc.Message) error
	Close() error
}
