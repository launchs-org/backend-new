package temporal

// CancelBuildInput は CancelBuildWorkflow への入力です。
// Backend と Builder の両方から参照できるように shared に定義します。
type CancelBuildInput struct {
	BuildJobID         string
	TemporalWorkflowID string
}
