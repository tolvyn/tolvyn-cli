package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// writeExport writes an export from r to path, readable only by its owner, and only once all of it arrived: it is
// written to a temporary file beside path and renamed over it at the end, so a failure part-way leaves nothing behind
// and an old file untouched. Without force an existing path is refused; its name is reserved first, so it cannot be
// taken in between. With force, a file or symlink at path is replaced by the rename, never written through, and
// anything else (a directory, a pipe, a device) is refused (BE-659).
func writeExport(path string, force bool, r io.Reader) (int64, error) {
	if st, err := os.Lstat(path); force && err == nil && !st.Mode().IsRegular() && st.Mode()&os.ModeSymlink == 0 {
		return 0, fmt.Errorf("%s is not a regular file or a symlink; --force replaces only those", path)
	}
	if !force {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			if os.IsExist(err) {
				return 0, fmt.Errorf("%s exists; pass --force to overwrite it", path)
			}
			return 0, err
		}
		f.Close()
	}
	n, err := writeThenRename(path, r)
	if err != nil && !force {
		os.Remove(path) //nolint:errcheck // the empty file reserved above
	}
	return n, err
}

func writeThenRename(path string, r io.Reader) (int64, error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tolvyn-export-*") // created 0600
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(tmp, r)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name()) //nolint:errcheck
	}
	return n, err
}
