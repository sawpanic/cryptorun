package atomicio

import (
	"io/fs"
	"os"
)

// WriteFile writes data to filename atomically using temp-then-rename pattern
// This ensures Windows-safe atomic writes without partial file states
func WriteFile(filename string, data []byte, perm fs.FileMode) error {
	tmp := filename + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, filename)
}