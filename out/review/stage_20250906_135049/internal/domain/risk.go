package domain

type PositionLimits struct{ MaxConcurrent int; SingleAssetPct float64 }

type CorrelationLimits struct{ MaxPerSector int; MaxPerEcosystem int }

func EnforcePositionLimits(current int, limits PositionLimits) bool {
	return current < limits.MaxConcurrent
}
