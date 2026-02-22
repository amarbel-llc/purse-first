package git

import (
	"strings"
)

func ParseRemoteList(output string) []RemoteEntry {
	if strings.TrimSpace(output) == "" {
		return []RemoteEntry{}
	}

	byName := make(map[string]*RemoteEntry)
	var order []string

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}

		name := parts[0]
		rest := parts[1]

		urlAndType := strings.SplitN(rest, " ", 2)
		url := urlAndType[0]
		typ := ""
		if len(urlAndType) > 1 {
			typ = strings.Trim(urlAndType[1], "()")
		}

		entry, ok := byName[name]
		if !ok {
			entry = &RemoteEntry{Name: name}
			byName[name] = entry
			order = append(order, name)
		}

		switch typ {
		case "fetch":
			entry.FetchURL = url
		case "push":
			entry.PushURL = url
		}
	}

	result := make([]RemoteEntry, 0, len(order))
	for _, name := range order {
		result = append(result, *byName[name])
	}

	return result
}
