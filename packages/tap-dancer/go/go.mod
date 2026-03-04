module github.com/amarbel-llc/purse-first/packages/tap-dancer/go

go 1.24.0

require (
	github.com/amarbel-llc/purse-first/libs/go-mcp v0.0.3-0.20260223142938-fd723e615485
	golang.org/x/text v0.34.0
)

require mvdan.cc/sh/v3 v3.12.0 // indirect

replace github.com/amarbel-llc/purse-first/libs/go-mcp => ../../../libs/go-mcp
