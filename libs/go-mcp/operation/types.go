package operation

type Outcome int

const (
	Success Outcome = iota
	Failure
	Skipped
	Aborted
)

type Annotation int

const (
	Idempotent Annotation = 1 << iota
	Destructive
	Recoverable
	ReadOnly
)

type Diagnostic struct {
	File     string
	Line     int
	Message  string
	Severity string
	Source   string
	Extras   map[string]any
}

type OperationEvent struct {
	Description string
	Annotations []Annotation
	Outcome     Outcome
	Diagnostic  *Diagnostic
	MustErrors  []error
}
