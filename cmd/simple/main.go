package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("🏃‍♂️ CryptoRun Simple CLI")
	
	if len(os.Args) > 1 && os.Args[1] == "scan" {
		runOfflineScan()
		return
	}
	
	fmt.Println(`
Usage:
  simple scan    Run offline scan with fake data
  
Options:
  scan          Execute momentum scanning with deterministic fake data
               Outputs to out/scan/latest_candidates.jsonl and scan_summary.json
`)
}

func runOfflineScan() {
	fmt.Println("🚀 Running Offline Scan")
	fmt.Println("Note: Run the following command manually:")
	fmt.Println("  go run test_offline_scan_with_output.go")
}