param([int]$TopN = 20)
$cmd = @'
Use WebFetch to pull 1h/24h/7d top gainers on Kraken (confirm USD pair exists).
Compare with ./out/scanner/latest_candidates.jsonl (gate traces).
Write ./out/analyst/winners.json, misses.jsonl, coverage.json, report.md.
Summarize recall and top 5 reason codes.
'@
& claude -a analyst_trader -p $cmd
