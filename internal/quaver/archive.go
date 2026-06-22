package quaver

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Source provides case-insensitive read access to a skin's files. Paths are
// forward-slash, relative to the skin root (e.g. "4k/HitObjects/note-hitobject-1.png").
type Source interface {
	// ReadFile returns the contents of path (case-insensitive lookup).
	ReadFile(path string) ([]byte, error)
	// Exists reports whether path is present (case-insensitive).
	Exists(path string) bool
	// List returns every file path (original case), unordered.
	List() []string
}

func normPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	return strings.TrimPrefix(p, "/")
}

// --- zip-backed source (.qs / .osk / .zip) ---

type zipSource struct {
	idx  map[string]*zip.File // lowercased path -> entry
	list []string
}

func (z *zipSource) Exists(p string) bool { _, ok := z.idx[strings.ToLower(normPath(p))]; return ok }
func (z *zipSource) List() []string       { return z.list }

func (z *zipSource) ReadFile(p string) ([]byte, error) {
	f, ok := z.idx[strings.ToLower(normPath(p))]
	if !ok {
		return nil, fs.ErrNotExist
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func newZipSource(data []byte) (*zipSource, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("read zip: %w", err)
	}
	z := &zipSource{idx: map[string]*zip.File{}}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		p := normPath(f.Name)
		z.idx[strings.ToLower(p)] = f
		z.list = append(z.list, p)
	}
	return z, nil
}

// --- directory-backed source ---

type dirSource struct {
	root string
	idx  map[string]string // lowercased rel path -> absolute path
	list []string
}

func (d *dirSource) Exists(p string) bool { _, ok := d.idx[strings.ToLower(normPath(p))]; return ok }
func (d *dirSource) List() []string       { return d.list }

func (d *dirSource) ReadFile(p string) ([]byte, error) {
	abs, ok := d.idx[strings.ToLower(normPath(p))]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return os.ReadFile(abs)
}

func newDirSource(root string) (*dirSource, error) {
	d := &dirSource{root: root, idx: map[string]string{}}
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = normPath(rel)
		d.idx[strings.ToLower(rel)] = path
		d.list = append(d.list, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return d, nil
}

// openSource opens a .qs/.osk/.zip archive or a skin folder as a Source.
func openSource(path string) (Source, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return newDirSource(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return newZipSource(data)
}
