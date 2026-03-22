package testutil

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// LoadFixture reads a fixture file from disk.
func LoadFixture(t testing.TB, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}

// BuildZipFixture builds a zip archive from all files under a directory.
func BuildZipFixture(t testing.TB, root string) []byte {
	t.Helper()

	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir(%q) error = %v", root, err)
	}

	sort.Strings(files)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, file := range files {
		rel, err := filepath.Rel(root, file)
		if err != nil {
			t.Fatalf("Rel(%q, %q) error = %v", root, file, err)
		}

		writer, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			t.Fatalf("Create(%q) error = %v", rel, err)
		}

		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", file, err)
		}
		if _, err := writer.Write(content); err != nil {
			t.Fatalf("Write(%q) error = %v", rel, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	return buf.Bytes()
}
