package osu

import (
	"archive/zip"
	"os"
	"path/filepath"
)

// WriteFolder writes files into dir. It builds into a sibling temp directory
// first, then swaps it into place, so a failure never leaves a half-written
// skin folder and an existing folder is replaced atomically.
func WriteFolder(dir string, files map[string][]byte) error {
	tmp := dir + ".qoconv-tmp"
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := writeAll(tmp, files); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	return os.Rename(tmp, dir)
}

func writeAll(dir string, files map[string][]byte) error {
	for name, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// WriteOSK writes files into a .osk (ZIP) archive at path.
func WriteOSK(path string, files map[string][]byte) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	zw := zip.NewWriter(f)
	for name, data := range files {
		w, werr := zw.Create(name)
		if werr != nil {
			zw.Close()
			return werr
		}
		if _, werr := w.Write(data); werr != nil {
			zw.Close()
			return werr
		}
	}
	return zw.Close()
}
