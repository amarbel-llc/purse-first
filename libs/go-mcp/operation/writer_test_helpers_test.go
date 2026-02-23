package operation

type nullWriter struct{}

func (nullWriter) BeginOperation(int, *OperationEvent) {}
func (nullWriter) EndOperation(int, *OperationEvent)   {}

type recordingWriter struct {
	begins []recordedEvent
	ends   []recordedEvent
}

type recordedEvent struct {
	depth int
	event OperationEvent
}

func (w *recordingWriter) BeginOperation(depth int, op *OperationEvent) {
	w.begins = append(w.begins, recordedEvent{depth: depth, event: *op})
}

func (w *recordingWriter) EndOperation(depth int, op *OperationEvent) {
	w.ends = append(w.ends, recordedEvent{depth: depth, event: *op})
}
