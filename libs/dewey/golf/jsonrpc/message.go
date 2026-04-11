// Package jsonrpc implements JSON-RPC 2.0 message handling.
// It provides types and utilities for building JSON-RPC clients and servers.
package jsonrpc

import (
	"encoding/json"
	"fmt"
)

const Version = "2.0"

type ID struct {
	num *int64
	str *string
}

func NewNumberID(n int64) ID {
	return ID{num: &n}
}

func NewStringID(s string) ID {
	return ID{str: &s}
}

func (id ID) IsNull() bool {
	return id.num == nil && id.str == nil
}

func (id ID) MarshalJSON() ([]byte, error) {
	if id.num != nil {
		return json.Marshal(*id.num)
	}
	if id.str != nil {
		return json.Marshal(*id.str)
	}
	return []byte("null"), nil
}

func (id *ID) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	var num int64
	if err := json.Unmarshal(data, &num); err == nil {
		id.num = &num
		return nil
	}

	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		id.str = &str
		return nil
	}

	return fmt.Errorf("id must be number, string, or null")
}

func (id ID) String() string {
	if id.num != nil {
		return fmt.Sprintf("%d", *id.num)
	}
	if id.str != nil {
		return *id.str
	}
	return "<null>"
}

type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *ID             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603

	ServerNotInitialized = -32002
	RequestCancelled     = -32800
	ContentModified      = -32801
)

func (m *Message) IsRequest() bool {
	return m.ID != nil && m.Method != ""
}

func (m *Message) IsNotification() bool {
	return m.ID == nil && m.Method != ""
}

func (m *Message) IsResponse() bool {
	return m.ID != nil && m.Method == ""
}

func NewRequest(id ID, method string, params any) (*Message, error) {
	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshaling params: %w", err)
		}
	}

	return &Message{
		JSONRPC: Version,
		ID:      &id,
		Method:  method,
		Params:  rawParams,
	}, nil
}

func NewNotification(method string, params any) (*Message, error) {
	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshaling params: %w", err)
		}
	}

	return &Message{
		JSONRPC: Version,
		Method:  method,
		Params:  rawParams,
	}, nil
}

func NewResponse(id ID, result any) (*Message, error) {
	var rawResult json.RawMessage
	if result != nil {
		var err error
		rawResult, err = json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshaling result: %w", err)
		}
	}

	return &Message{
		JSONRPC: Version,
		ID:      &id,
		Result:  rawResult,
	}, nil
}

func NewErrorResponse(id ID, code int, message string, data any) (*Message, error) {
	var rawData json.RawMessage
	if data != nil {
		var err error
		rawData, err = json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("marshaling error data: %w", err)
		}
	}

	return &Message{
		JSONRPC: Version,
		ID:      &id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    rawData,
		},
	}, nil
}
