package tap

// Severity indicates the severity of a validation diagnostic.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return "unknown"
	}
}

// Diagnostic represents a single validation problem found in TAP input.
type Diagnostic struct {
	Line     int      `json:"line"`
	Severity Severity `json:"severity"`
	Rule     string   `json:"rule"`
	Message  string   `json:"message"`
}

// Directive represents a TAP test point directive.
type Directive int

const (
	DirectiveNone Directive = iota
	DirectiveSkip
	DirectiveTodo
)

func (d Directive) String() string {
	switch d {
	case DirectiveSkip:
		return "SKIP"
	case DirectiveTodo:
		return "TODO"
	default:
		return ""
	}
}

// EventType classifies a parsed TAP line.
type EventType int

const (
	EventVersion EventType = iota
	EventPlan
	EventTestPoint
	EventYAMLDiagnostic
	EventComment
	EventBailOut
	EventPragma
	EventSubtestStart
	EventSubtestEnd
	EventUnknown
)

// TestPointResult holds parsed data from a test point line.
type TestPointResult struct {
	Number      int       `json:"number"`
	Description string    `json:"description"`
	OK          bool      `json:"ok"`
	Directive   Directive `json:"directive"`
	Reason      string    `json:"reason,omitempty"`
}

// PlanResult holds parsed data from a plan line.
type PlanResult struct {
	Count  int    `json:"count"`
	Reason string `json:"reason,omitempty"`
}

// BailOutResult holds parsed data from a bail out line.
type BailOutResult struct {
	Reason string `json:"reason,omitempty"`
}

// PragmaResult holds parsed data from a pragma line.
type PragmaResult struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

// Event represents a single parsed TAP element.
type Event struct {
	Type      EventType         `json:"type"`
	Line      int               `json:"line"`
	Depth     int               `json:"depth"`
	Raw       string            `json:"raw"`
	TestPoint *TestPointResult  `json:"test_point,omitempty"`
	Plan      *PlanResult       `json:"plan,omitempty"`
	BailOut   *BailOutResult    `json:"bail_out,omitempty"`
	YAML      map[string]string `json:"yaml,omitempty"`
	Comment   string            `json:"comment,omitempty"`
	Pragma    *PragmaResult     `json:"pragma,omitempty"`
}

// Summary provides aggregate results after parsing a TAP document.
type Summary struct {
	Version    int  `json:"version"`
	TotalTests int  `json:"total_tests"`
	Passed     int  `json:"passed"`
	Failed     int  `json:"failed"`
	Skipped    int  `json:"skipped"`
	Todo       int  `json:"todo"`
	BailedOut  bool `json:"bailed_out"`
	PlanCount  int  `json:"plan_count"`
	Valid      bool `json:"valid"`
}
