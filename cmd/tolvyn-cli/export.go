package main

import (
	"fmt"
	"os"
)

// createExportFile opens an export for writing, readable only by its owner. An existing file is refused unless force
// is set; with force it is truncated and made private (BE-659).
func createExportFile(path string, force bool) (*os.File, error) {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("%s exists; pass --force to overwrite it", path)
		}
		return nil, err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
