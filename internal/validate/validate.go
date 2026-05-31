package validate

import (
	"fmt"
	"os"
	"path/filepath"
)

func ValidateBytes(data []byte, docType DocType, strict bool) (*Result, DocType, error) {
	if docType == Unknown {
		docType = DetectTypeFromContent(data)
	}

	if docType == Unknown {
		return nil, Unknown, fmt.Errorf("cannot determine document type; use --type to specify")
	}

	var r *Result
	switch docType {
	case PluginDoc:
		r = validatePlugin(data, strict)
	case MappingDoc:
		r = validateMapping(data, strict)
	default:
		return nil, docType, fmt.Errorf("unsupported document type: %s", docType)
	}

	return r, docType, nil
}

func ValidateFile(path string, docType DocType, strict bool) (*Result, DocType, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, Unknown, fmt.Errorf("reading %s: %w", path, err)
	}

	if docType == Unknown {
		docType = DetectTypeFromFilename(path)
	}

	return ValidateBytes(data, docType, strict)
}

var directoryTargets = []string{
	".claude-plugin/plugin.json",
	"mappings.json",
}

func ValidateDirectory(dir string, strict bool) (*Result, error) {
	combined := &Result{}
	found := false

	for _, target := range directoryTargets {
		path := filepath.Join(dir, target)
		if _, err := os.Stat(path); err != nil {
			continue
		}

		found = true

		r, docType, err := ValidateFile(path, Unknown, strict)
		if err != nil {
			return nil, fmt.Errorf("validating %s: %w", path, err)
		}

		for _, issue := range r.Issues() {
			combined.issues = append(combined.issues, Issue{
				Severity: issue.Severity,
				Path:     fmt.Sprintf("%s: %s", target, issue.Path),
				Message:  issue.Message,
			})
		}

		_ = docType
	}

	if !found {
		return nil, fmt.Errorf("no recognized documents found in %s", dir)
	}

	return combined, nil
}
