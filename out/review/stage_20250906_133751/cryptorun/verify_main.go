package main

import (
	"context"

	"cryptorun/internal/application"
)

// handleVerificationSweep handles the verification sweep (read-only)
func (ui *MenuUI) handleVerificationSweep(ctx context.Context) error {
	sweep := application.NewVerificationSweep()
	return sweep.RunVerification(ctx)
}
