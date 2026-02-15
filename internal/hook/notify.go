package hook

import "github.com/amarbel-llc/purse-first/internal/mcp"

func fireNotificationsForEvent(event string, plugins []mcp.ServerEntry, vars map[string]string) {
	// Notifications are no longer carried in plugin manifests.
	// Each plugin handles its own hooks through Claude Code's native plugin system.
}
