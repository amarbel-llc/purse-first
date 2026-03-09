package executor

import (
	"os/exec"
	"strings"
)

func parseSessions(output string) map[string]bool {
	sessions := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// line may start with "→ " for current session
		line = strings.TrimPrefix(line, "→ ")
		line = strings.TrimSpace(line)
		for _, field := range strings.Split(line, "\t") {
			field = strings.TrimSpace(field)
			if strings.HasPrefix(field, "session_name=") {
				key := strings.TrimPrefix(field, "session_name=")
				if key != "" {
					sessions[key] = true
				}
			}
		}
	}
	return sessions
}

func ListSessions() map[string]bool {
	cmd := exec.Command("zmx", "-g", "sc", "list")
	out, err := cmd.Output()
	if err != nil {
		return make(map[string]bool)
	}
	return parseSessions(string(out))
}
