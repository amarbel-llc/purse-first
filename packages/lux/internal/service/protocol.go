package service

import "encoding/json"

const (
	MethodSessionRegister   = "lux/session.register"
	MethodSessionDeregister = "lux/session.deregister"
	MethodLSPRequest        = "lux/lsp.request"
	MethodLSPNotification   = "lux/lsp.notification"
	MethodPoolStatus        = "lux/pool.status"
	MethodPoolStart         = "lux/pool.start"
	MethodPoolStop          = "lux/pool.stop"
	MethodWarmup            = "lux/warmup"
)

type ClientType string

const (
	ClientTypeLSP     ClientType = "lsp"
	ClientTypeMCP     ClientType = "mcp"
	ClientTypeControl ClientType = "control"
)

type RegisterParams struct {
	WorkspaceRoot string     `json:"workspace_root"`
	ClientType    ClientType `json:"client_type"`
}

type RegisterResult struct {
	SessionID string `json:"session_id"`
}

type DeregisterParams struct {
	SessionID string `json:"session_id"`
}

type LSPRequestParams struct {
	SessionID string          `json:"session_id"`
	LSPMethod string          `json:"lsp_method"`
	LSPParams json.RawMessage `json:"lsp_params"`
}

type LSPNotificationParams struct {
	SessionID string          `json:"session_id"`
	LSPMethod string          `json:"lsp_method"`
	LSPParams json.RawMessage `json:"lsp_params"`
}

type PoolStartParams struct {
	Name string `json:"name"`
}

type PoolStopParams struct {
	Name string `json:"name"`
}

type WarmupParams struct {
	Dir string `json:"dir"`
}
