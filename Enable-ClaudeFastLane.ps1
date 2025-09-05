claude -a builder --permission-mode plan -p @"
TASK:
Unblock build & tests end-to-end: remove duplicate PrintTable in ui/table.go, fix Binance book type mismatch, tidy modules, and get green tests.

SCOPE:
- Edit only: go.mod, go.sum, src/ui/table.go, src/exchanges/binance/book.go, tests/unit/*
- No other files.

ACCEPTANCE:
- go mod tidy && go mod download succeed
- go build -tags no_net ./... passes
- go test ./... -count=1 passes
- One public PrintTable remains; binance book parses sample book; simple test added

LIMITS:
- Max files: 6
- Max lines: 400
- Time budget: 20 minutes

PLAN:
1) go mod tidy/download; go get missing modules if needed.
2) ui/table.go: remove duplicate PrintTable; consolidate helpers.
3) binance/book.go: align types & conversions.
4) tests/unit: add minimal tests for both.
5) Build/test; print Created|Modified|Skipped.

DIFF TABLE:
- M go.mod
- M go.sum
- M src/ui/table.go
- M src/exchanges/binance/book.go
- A tests/unit/table_print_test.go
- A tests/unit/binance_book_parse_test.go
"@
