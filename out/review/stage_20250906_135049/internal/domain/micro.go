package domain

// Microstructure stub types for build compatibility

type GateResult struct {
	OK        bool
	Value     float64
	Threshold float64
	Name      string
}

type MicroGateThresholds struct {
	MaxSpreadBps float64
	MinDepthUSD  float64
	MinVADR      float64
	MinADVUSD    int64
}

type MicroGateResults struct{
	AllPass bool
	Reason  string
	Symbol  string
	Spread  GateResult
	Depth   GateResult
	VADR    GateResult
	ADV     GateResult
}
type MicroGateInputs struct {
	Symbol      string
	Bid         float64
	Ask         float64
	Depth2PcUSD float64
	VADR        float64
	ADVUSD      int64
}

func DefaultMicroGateThresholds() MicroGateThresholds { return MicroGateThresholds{} }
func EvaluateMicroGates(MicroGateInputs, MicroGateThresholds) MicroGateResults { return MicroGateResults{} }
func CalculateSpreadBps(bid, ask float64) float64 {
	if bid <= 0 || ask <= 0 || ask <= bid {
		return 0
	}
	midpoint := (bid + ask) / 2
	if midpoint <= 0 {
		return 0
	}
	return ((ask - bid) / midpoint) * 10000
}