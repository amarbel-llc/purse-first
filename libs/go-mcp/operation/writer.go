package operation

type Writer interface {
	BeginOperation(depth int, op *OperationEvent)
	EndOperation(depth int, op *OperationEvent)
}
