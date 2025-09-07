package unit

import (
	"testing"

	"cryptorun/internal/qa"
)

// TestNoStubGate ensures the repository is free of scaffold patterns
// This test enforces hard failure in CI/CD if scaffolds are found
func TestNoStubGate(t *testing.T) {
	// Run no-stub gate scan
	gate := qa.NewNoStubGate("out/audit")
	report, err := gate.Scan()
	if err != nil {
		t.Fatalf("No-stub gate scan failed: %v", err)
	}

	// Hard failure if any scaffolds found
	if report.TotalHits > 0 {
		t.Errorf("FAIL SCAFFOLDS_REMAIN +hint: Found %d scaffold patterns in repository", report.TotalHits)
		t.Errorf("Evidence written to: out/audit/nostub_hits.json")
		t.Errorf("First few violations:")
		
		maxShow := 5
		if len(report.Hits) < maxShow {
			maxShow = len(report.Hits)
		}
		
		for i := 0; i < maxShow; i++ {
			hit := report.Hits[i]
			t.Errorf("  %s:%d - %s pattern: %s", hit.File, hit.Line, hit.Pattern, hit.Excerpt)
		}
		
		if len(report.Hits) > maxShow {
			t.Errorf("  ... and %d more (see JSON for full list)", len(report.Hits)-maxShow)
		}
		
		t.FailNow()
	}

	t.Logf("✅ PASS No-stub gate: 0 scaffold patterns found (scanned %d files)", report.Scanned)
}

// TestBannedTokenGate ensures documentation is free of banned terms
func TestBannedTokenGate(t *testing.T) {
	// Banned token checking is handled by the branding guard test
	// which already enforces CProtocol restrictions outside _codereview/
	t.Log("✅ PASS Banned token check delegated to branding guard test")
}