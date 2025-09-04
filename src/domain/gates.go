package domain

type GateInputs struct { DailyVolUSD float64; VADR float64; SpreadBps float64 }

type EntryDecision struct { Allow bool; Reason string }

func EntryGates(inp GateInputs) EntryDecision {
	if inp.DailyVolUSD < 200000 { return EntryDecision{false, "kraken volume < $200k"} }
	// additional 6 gates: placeholders pass-through
	return EntryDecision{true, "ok"}
}
