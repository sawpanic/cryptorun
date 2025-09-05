package application

type BudgetState struct{ MonthlyLimitUSD, UsedUSD, SwitchAtRemainingUSD int }

func (b BudgetState) ShouldSwitch() bool {
	remaining := b.MonthlyLimitUSD - b.UsedUSD
	return remaining <= b.SwitchAtRemainingUSD
}
