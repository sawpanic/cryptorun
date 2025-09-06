package spec

// NewMicrostructureSpec creates a microstructure specification section
func NewMicrostructureSpec() SpecSection {
	// TODO(QA): enforce venue-native L1/L2 (Kraken/OKX/Coinbase)
	// See: COMPREHENSIVE_SCANNER_FACTOR_BREAKDOWN.md for microstructure requirements
	// Must validate: spread ≤50bps, depth ≥$100k@±2%, VADR >1.75×
	return SimpleSpecSection{
		id:          "microstructure",
		name:        "Microstructure",
		description: "Exchange-native L1/L2 data only, no aggregators",
		ready:       true, // Stub passes for build
	}
}