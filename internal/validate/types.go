package validate

import "fmt"

type Severity int

const (
	Error Severity = iota
	Warning
	Info
)

func (s Severity) String() string {
	switch s {
	case Warning:
		return "warning"
	case Info:
		return "info"
	default:
		return "error"
	}
}

type Issue struct {
	Severity Severity
	Path     string
	Message  string
}

func (i Issue) String() string {
	if i.Path != "" {
		return fmt.Sprintf("%s: %s: %s", i.Severity, i.Path, i.Message)
	}
	return fmt.Sprintf("%s: %s", i.Severity, i.Message)
}

type Result struct {
	issues []Issue
}

func (r *Result) addError(path, msg string) {
	r.issues = append(r.issues, Issue{Severity: Error, Path: path, Message: msg})
}

func (r *Result) addWarning(path, msg string) {
	r.issues = append(r.issues, Issue{Severity: Warning, Path: path, Message: msg})
}

func (r *Result) addInfo(path, msg string) {
	r.issues = append(r.issues, Issue{Severity: Info, Path: path, Message: msg})
}

func (r *Result) HasErrors() bool {
	for _, i := range r.issues {
		if i.Severity == Error {
			return true
		}
	}
	return false
}

func (r *Result) HasWarnings() bool {
	for _, i := range r.issues {
		if i.Severity == Warning {
			return true
		}
	}
	return false
}

func (r *Result) Errors() []Issue {
	var out []Issue
	for _, i := range r.issues {
		if i.Severity == Error {
			out = append(out, i)
		}
	}
	return out
}

func (r *Result) Warnings() []Issue {
	var out []Issue
	for _, i := range r.issues {
		if i.Severity == Warning {
			out = append(out, i)
		}
	}
	return out
}

func (r *Result) Issues() []Issue {
	return r.issues
}
