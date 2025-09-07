
# ORTHOGONAL FACTOR MONITORING DASHBOARD

## Alert Thresholds:
- Max correlation within cluster: 0.6 (alert at 0.7)
- VIF per feature: 5.0 (alert at 6.0)
- IC stability: 95% CI overlap with 0 for 4 weeks
- PSI drift: 0.25 (quarantine factor)

## Automatic Actions:
- Correlation breach → Factor residualization review
- VIF breach → Feature selection review  
- IC instability → Factor demotion
- PSI drift → Factor quarantine

## Rollback Triggers:
- Slippage > 2× model for 3 consecutive days
- VaR threshold breach
- Max drawdown exceeded
