package commands

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// RemoteEntry represents a file or directory on a remote host
type RemoteEntry struct {
	Name    string
	Path    string
	IsDir   bool
	Size    int64
	ModTime time.Time
	Marked  bool
}

// GetRemoteEntries lists files on a remote host via SSH + ls -la
func GetRemoteEntries(ctx context.Context, host *SSHHost, dirPath string) ([]*RemoteEntry, error) {
	port := host.Port
	if port == 0 {
		port = 22
	}
	portStr := fmt.Sprintf("%d", port)

	args := []string{}
	if host.KeyPath != "" {
		args = append(args, "-i", host.KeyPath)
	}
	args = append(args, "-p", portStr)
	args = append(args, "-o", "StrictHostKeyChecking=no")
	args = append(args, "-o", "BatchMode=yes")
	args = append(args, fmt.Sprintf("%s@%s", host.User, host.Hostname))
	args = append(args, fmt.Sprintf("ls -la %s", dirPath))

	cmd := exec.CommandContext(ctx, "ssh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ssh ls failed on %s:%s: %w\n%s", host.Hostname, dirPath, err, string(output))
	}

	entries := ParseLSOutput(string(output), dirPath)
	sort.Slice(entries, func(i, j int) bool {
		// Directories first, then alphabetical
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	return entries, nil
}

// ParseLSOutput parses standard `ls -la` output into RemoteEntry structs.
// Expected format per line: "drwxr-xr-x  2 user group  4096 Jan  5 12:00 dirname"
func ParseLSOutput(output string, basePath string) []*RemoteEntry {
	var entries []*RemoteEntry

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip the "total" line
		if strings.HasPrefix(line, "total ") {
			continue
		}

		entry := parseLSLine(line, basePath)
		if entry == nil {
			continue
		}
		// Skip . and ..
		if entry.Name == "." || entry.Name == ".." {
			continue
		}
		entries = append(entries, entry)
	}

	return entries
}

// parseLSLine parses a single ls -la line into a RemoteEntry.
// Format: perms links owner group size month day time/year name
func parseLSLine(line string, basePath string) *RemoteEntry {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return nil
	}

	perms := fields[0]
	isDir := len(perms) > 0 && perms[0] == 'd'

	size, _ := strconv.ParseInt(fields[4], 10, 64)

	// Parse modification time from month day time/year (fields 5, 6, 7)
	modTime := parseModTime(fields[5], fields[6], fields[7])

	// Name is everything after the 8th field (handles spaces in filenames)
	name := strings.Join(fields[8:], " ")

	// For symlinks, ls -la shows "name -> target"; keep only the name
	if idx := strings.Index(name, " -> "); idx != -1 {
		name = name[:idx]
	}

	return &RemoteEntry{
		Name:    name,
		Path:    filepath.Join(basePath, name),
		IsDir:   isDir,
		Size:    size,
		ModTime: modTime,
	}
}

// parseModTime attempts to parse the date fields from ls -la output.
// Handles two formats: "Jan  5 12:00" (recent) and "Jan  5  2023" (old).
func parseModTime(month, day, timeOrYear string) time.Time {
	now := time.Now()
	dateStr := fmt.Sprintf("%s %s %s", month, day, timeOrYear)

	// Try "Jan 2 15:04" format (recent files)
	if strings.Contains(timeOrYear, ":") {
		t, err := time.Parse("Jan 2 15:04", dateStr)
		if err == nil {
			// ls shows no year for recent files; reconstruct with current year
			t = time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)
			// If parsed time is in the future, it's from last year
			if t.After(now) {
				t = t.AddDate(-1, 0, 0)
			}
			return t
		}
	}

	// Try "Jan 2 2006" format (older files)
	t, err := time.Parse("Jan 2 2006", dateStr)
	if err == nil {
		return t
	}

	return time.Time{}
}

// GetRemoteIcon returns a display icon for the entry
func GetRemoteIcon(entry *RemoteEntry) string {
	if entry.IsDir {
		return "📁"
	}
	return "📄"
}
