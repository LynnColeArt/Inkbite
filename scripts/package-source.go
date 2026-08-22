//go:build ignore

// Command package-source writes deterministic source tar.gz and ZIP archives
// without depending on platform-specific tar creation flags.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

var archiveTime = time.Unix(946684800, 0).UTC()

type archiveEntry struct {
	diskPath string
	name     string
	dir      bool
	size     int64
}

func main() {
	if len(os.Args) != 5 {
		fatalf("usage: package-source ROOT ARCHIVE_NAME TAR_GZ ZIP")
	}
	entries, err := collectEntries(os.Args[1], os.Args[2])
	if err != nil {
		fatalf("collect source archive: %v", err)
	}
	if err := writeTarGzip(os.Args[3], entries); err != nil {
		fatalf("write source tar.gz: %v", err)
	}
	if err := writeZIP(os.Args[4], entries); err != nil {
		fatalf("write source zip: %v", err)
	}
}

func collectEntries(root, archiveName string) ([]archiveEntry, error) {
	root = filepath.Clean(root)
	var entries []archiveEntry
	err := filepath.WalkDir(root, func(path string, dirEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := dirEntry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return fmt.Errorf("unsupported source entry %q", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := archiveName
		if relative != "." {
			name += "/" + filepath.ToSlash(relative)
		}
		if info.IsDir() {
			name += "/"
		}
		entries = append(entries, archiveEntry{
			diskPath: path,
			name:     name,
			dir:      info.IsDir(),
			size:     info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	return entries, nil
}

func writeTarGzip(path string, entries []archiveEntry) (resultErr error) {
	output, err := os.Create(path)
	if err != nil {
		return err
	}
	defer closeWithError(output, &resultErr)

	compressed := gzip.NewWriter(output)
	compressed.Header.ModTime = time.Time{}
	compressed.Header.OS = 255
	defer closeWithError(compressed, &resultErr)

	archive := tar.NewWriter(compressed)
	defer closeWithError(archive, &resultErr)
	for _, entry := range entries {
		header := &tar.Header{
			Name:       entry.name,
			Mode:       0o644,
			Size:       entry.size,
			ModTime:    archiveTime,
			AccessTime: archiveTime,
			ChangeTime: archiveTime,
			Uid:        0,
			Gid:        0,
			Format:     tar.FormatPAX,
		}
		if entry.dir {
			header.Typeflag = tar.TypeDir
			header.Mode = 0o755
			header.Size = 0
		} else {
			header.Typeflag = tar.TypeReg
		}
		if err := archive.WriteHeader(header); err != nil {
			return err
		}
		if entry.dir {
			continue
		}
		if err := copyFile(archive, entry.diskPath); err != nil {
			return err
		}
	}
	return nil
}

func writeZIP(path string, entries []archiveEntry) (resultErr error) {
	output, err := os.Create(path)
	if err != nil {
		return err
	}
	defer closeWithError(output, &resultErr)

	archive := zip.NewWriter(output)
	defer closeWithError(archive, &resultErr)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.SetModTime(archiveTime)
		if entry.dir {
			header.Method = zip.Store
			header.SetMode(os.ModeDir | 0o755)
		} else {
			header.SetMode(0o644)
		}
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		if entry.dir {
			continue
		}
		if err := copyFile(writer, entry.diskPath); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(destination io.Writer, path string) (resultErr error) {
	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer closeWithError(source, &resultErr)
	_, err = io.Copy(destination, source)
	return err
}

func closeWithError(closer io.Closer, resultErr *error) {
	if err := closer.Close(); *resultErr == nil && err != nil {
		*resultErr = err
	}
}

func fatalf(format string, values ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}
