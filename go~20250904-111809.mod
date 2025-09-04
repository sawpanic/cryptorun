module github.com/cryptoedge/cryptoedge

replace github.com/cryptoedge/internal => ../internal

go 1.21

require (
	github.com/cryptoedge/internal v0.0.0-00010101000000-000000000000
	github.com/fatih/color v1.18.0
	github.com/shopspring/decimal v1.4.0
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
)
