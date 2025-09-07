package gates

// EntryGateEvaluator evaluates entry gates for trading candidates
type EntryGateEvaluator struct{}

// NewEntryGateEvaluator creates a new entry gate evaluator
func NewEntryGateEvaluator(a, b, c, d interface{}) *EntryGateEvaluator {
	return &EntryGateEvaluator{}
}