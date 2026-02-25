module github.com/amarbel-llc/lux

go 1.25.6

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/amarbel-llc/purse-first/libs/go-mcp v0.0.3-0.20260223142938-fd723e615485
	github.com/gobwas/glob v0.2.3
)

require mvdan.cc/sh/v3 v3.12.0 // indirect

replace github.com/amarbel-llc/purse-first/libs/go-mcp => ../../libs/go-mcp
