package hook

import (
	"io"

	"github.com/friedenberg/purse-first/internal/mcp"
)

func HandleSessionEnd(stdin io.Reader, stdout io.Writer) error {
	plugins, err := mcp.DiscoverPlugins()
	if err != nil || len(plugins) == 0 {
		return nil
	}

	fireNotificationsForEvent("stop", plugins, nil)

	return nil
}
