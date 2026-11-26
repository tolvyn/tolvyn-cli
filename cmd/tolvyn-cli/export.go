package main

import (
	"fmt"
	"os"
)

// createExportFile creates an export, readable only by its owner. An existing file is refused unless force is set;
// with force a file or symlink at the path is removed first, never written through (BE-659).
func createExportFile(path string, force bool) (*os.File, error) {
	if st, err := os.Lstat(path); force && err == nil && (st.Mode().IsRegular() || st.Mode()&os.ModeSymlink != 0) {
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
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
