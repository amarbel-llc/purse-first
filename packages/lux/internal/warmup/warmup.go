package warmup

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/lsp"
	"github.com/amarbel-llc/lux/internal/subprocess"
)

func PreBuildAll(ctx context.Context, cfg *config.Config, executor subprocess.Executor) {
	var wg sync.WaitGroup
	for _, l := range cfg.LSPs {
		wg.Add(1)
		go func(flake, binary, name string) {
			defer wg.Done()
			if _, err := executor.Build(ctx, flake, binary); err != nil {
				fmt.Fprintf(os.Stderr, "[lux] pre-build %s: %v\n", name, err)
			}
		}(l.Flake, l.Binary, l.Name)
	}
	wg.Wait()
}

func StartRelevantLSPs(ctx context.Context, pool *subprocess.Pool, scanner *Scanner,
	dirs []string, initParams *lsp.InitializeParams, cfg *config.Config) {
	result := scanner.ScanDirectories(dirs)

	// Merge in eager_start LSPs
	for _, l := range cfg.LSPs {
		if l.ShouldEagerStart() {
			result.LSPNames[l.Name] = true
		}
	}

	var wg sync.WaitGroup
	for name := range result.LSPNames {
		if !pool.IsIdleOrFailed(name) {
			continue
		}
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			if _, err := pool.GetOrStart(ctx, n, initParams); err != nil {
				fmt.Fprintf(os.Stderr, "[lux] eager start %s: %v\n", n, err)
			}
		}(name)
	}
	wg.Wait()
}

func StartAllLSPs(ctx context.Context, pool *subprocess.Pool, cfg *config.Config,
	initParams *lsp.InitializeParams) {
	var wg sync.WaitGroup
	for _, l := range cfg.LSPs {
		if !pool.IsIdleOrFailed(l.Name) {
			continue
		}
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			if _, err := pool.GetOrStart(ctx, name, initParams); err != nil {
				fmt.Fprintf(os.Stderr, "[lux] eager start %s: %v\n", name, err)
			}
		}(l.Name)
	}
	wg.Wait()
}
