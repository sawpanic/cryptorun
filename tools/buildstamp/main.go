package main

import (
    "fmt"
    "time"
)

func main() {
    // Prefer IANA tz for Jerusalem; fallback to UTC if unavailable
    loc, err := time.LoadLocation("Asia/Jerusalem")
    if err != nil {
        fmt.Printf(time.Now().UTC().Format("2006-01-02 15:04") + " UTC")
        return
    }
    now := time.Now().In(loc)
    fmt.Printf(now.Format("2006-01-02 15:04") + " Jerusalem")
}

