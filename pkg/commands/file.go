package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FileEntry represents a file or directory in the local filesystem
type FileEntry struct {
	Name    string
	Path    string
	IsDir   bool
	Size    int64
	ModTime time.Time
	Marked  bool // for SCP selection
}

// GetFileEntries lists the contents of a directory and returns sorted FileEntry slices
func GetFileEntries(dirPath string) ([]*FileEntry, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	entries := make([]*FileEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		info, err := de.Info()
		if err != nil {
			// Skip files we can't stat (permission errors, etc.)
			continue
		}

		entries = append(entries, &FileEntry{
			Name:    de.Name(),
			Path:    filepath.Join(dirPath, de.Name()),
			IsDir:   de.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		// Directories first, then alphabetical
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	return entries, nil
}

// GetFileIcon returns an icon string for the file entry
func GetFileIcon(entry *FileEntry) string {
	if entry.IsDir {
		return "📁"
	}
	return "📄"
}

// IsHiddenFile returns true if the file name starts with a dot
func IsHiddenFile(name string) bool {
	return strings.HasPrefix(name, ".")
}

// GetFilePermissions returns the permission string in rwxrwxrwx format
func GetFilePermissions(fi os.FileInfo) string {
	mode := fi.Mode()
	perms := [9]byte{'-', '-', '-', '-', '-', '-', '-', '-', '-'}

	if mode&0400 != 0 {
		perms[0] = 'r'
	}
	if mode&0200 != 0 {
		perms[1] = 'w'
	}
	if mode&0100 != 0 {
		perms[2] = 'x'
	}
	if mode&0040 != 0 {
		perms[3] = 'r'
	}
	if mode&0020 != 0 {
		perms[4] = 'w'
	}
	if mode&0010 != 0 {
		perms[5] = 'x'
	}
	if mode&0004 != 0 {
		perms[6] = 'r'
	}
	if mode&0002 != 0 {
		perms[7] = 'w'
	}
	if mode&0001 != 0 {
		perms[8] = 'x'
	}

	return string(perms[:])
}
