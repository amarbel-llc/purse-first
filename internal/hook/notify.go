package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/friedenberg/purse-first/internal/mcp"
)

func fireNotification(n mcp.Notification, vars map[string]string) {
	if n.When != nil {
		filePath := vars["file_path"]
		if n.When.HasFilePath && filePath == "" {
			return
		}
		if n.When.FilePathAbsolute && !strings.HasPrefix(filePath, "/") {
			return
		}
	}

	body := n.HTTPPost.Body
	if n.HTTPPost.BodyTemplate != nil {
		body = applyTemplate(n.HTTPPost.BodyTemplate, vars)
	}

	data, err := json.Marshal(body)
	if err != nil {
		return
	}

	url := buildURL(n.HTTPPost)

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		// Fail open: server not running is fine
		return
	}
	defer resp.Body.Close()
}

func buildURL(action mcp.HTTPPostAction) string {
	port := ""
	if action.PortEnv != "" {
		port = os.Getenv(action.PortEnv)
	}
	if port == "" {
		port = fmt.Sprintf("%d", action.DefaultPort)
	}
	return fmt.Sprintf("http://localhost:%s%s", port, action.Path)
}

func applyTemplate(tmpl map[string]any, vars map[string]string) map[string]any {
	result := make(map[string]any, len(tmpl))
	for k, v := range tmpl {
		if s, ok := v.(string); ok {
			for varName, varVal := range vars {
				s = strings.ReplaceAll(s, "{"+varName+"}", varVal)
			}
			result[k] = s
		} else {
			result[k] = v
		}
	}
	return result
}

func fireNotificationsForEvent(event string, plugins []mcp.ServerEntry, vars map[string]string) {
	for _, plugin := range plugins {
		for _, n := range plugin.Notifications {
			if n.On == event {
				fireNotification(n, vars)
			}
		}
	}
}
