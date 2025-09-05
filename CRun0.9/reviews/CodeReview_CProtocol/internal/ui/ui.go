package ui

import (
    "fmt"
    "time"
)

// Minimal console UI stubs to satisfy dependencies and keep output readable.

type BufferManager struct{}

func GetBufferManager() *BufferManager { return &BufferManager{} }
func (b *BufferManager) AutoFlush(_ time.Duration) {}

func ShowBanner() {}

func SafePrintf(format string, a ...interface{}) { fmt.Printf(format, a...) }
func SafePrintln(a ...interface{})                 { fmt.Println(a...) }
func FlushOutput()                                 {}
func PrintError(msg string)                        { fmt.Println("ERROR:", msg) }
func PrintHeader(title string)                     { fmt.Println("==", title, "==") }
func PrintSuccess(msg string)                      { fmt.Println(msg) }

// DisplayCoordinator is a no-op wrapper to keep structure compatible
type DisplayCoordinator struct{}

func GetDisplayCoordinator() *DisplayCoordinator { return &DisplayCoordinator{} }
func (d *DisplayCoordinator) StartOperation(_id, _name string, _protect bool) {}
func (d *DisplayCoordinator) CompleteOperation(_id string)                   {}

// LiveProgressTracker is a simple progress printer
type LiveProgressTracker struct{}

func NewLiveProgressTracker() *LiveProgressTracker { return &LiveProgressTracker{} }
func (l *LiveProgressTracker) StartStep(step int)  { fmt.Printf("[Step %d] Start\n", step) }
func (l *LiveProgressTracker) CompleteStep(step int, _ map[string]interface{}, desc string) {
    fmt.Printf("[Step %d] %s\n", step, desc)
}

