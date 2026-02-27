package gui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"lazyscpsync/pkg/commands"
)

// renderHostsPanelContent renders the left SSH hosts panel content (no border/title)
func (m Model) renderHostsPanelContent(width, height int) string {
	isFocused := m.focusedSection == 0
	selected := m.selectedInSection[0]
	items := m.sections[0].Items

	if len(items) == 0 {
		return NewStyle().CyanForeground().Render("(empty)")
	}

	var lines []string

	// Apply scroll offset
	scrollOffset := m.hostsScroll
	endIdx := scrollOffset + height
	if endIdx > len(items) {
		endIdx = len(items)
	}

	for i := scrollOffset; i < endIdx; i++ {
		item := items[i]

		// Determine reachability status dot
		dot := "🟡" // not yet checked
		if host, ok := item.Data.(*commands.SSHHost); ok {
			if reachable, checked := m.hostReachability[host.Name]; checked {
				if reachable {
					dot = "🟢"
				} else {
					dot = "🔴"
				}
			}
		}

		label := dot + " " + item.Label

		if i == selected && isFocused {
			lines = append(lines, NewStyle().GreenBg().Bold().Render("▶"+label))
		} else {
			// Grey out unreachable hosts
			unreachable := false
			if host, ok := item.Data.(*commands.SSHHost); ok {
				if reachable, checked := m.hostReachability[host.Name]; checked && !reachable {
					unreachable = true
				}
			}
			if unreachable {
				lines = append(lines, NewStyle().GreyForeground().Render(" "+label))
			} else {
				lines = append(lines, NewStyle().CyanForeground().Render(" "+label))
			}
		}
	}

	// Scroll indicator
	if len(items) > height {
		indicator := fmt.Sprintf(" ↕ %d/%d", selected+1, len(items))
		if len(lines) >= height {
			lines[height-1] = NewStyle().YellowForeground().Render(indicator)
		} else {
			lines = append(lines, NewStyle().YellowForeground().Render(indicator))
		}
	}

	return strings.Join(lines, "\n")
}

// renderFileBrowserPanelContent renders a file browser panel content (no border/title)
func (m Model) renderFileBrowserPanelContent(sectionIdx, width, height int, title string) string {
	isFocused := m.focusedSection == sectionIdx
	selected := m.selectedInSection[sectionIdx]

	var currentPath string
	if sectionIdx == 1 {
		currentPath = m.localPath
	} else if sectionIdx == 2 {
		currentPath = m.remotePath
	}

	var lines []string

	// Path line
	pathLine := NewStyle().CyanForeground().Render("📁 " + currentPath)
	lines = append(lines, pathLine)

	// Show loading indicator for remote browser
	if sectionIdx == 2 && m.isLoadingRemote {
		lines = append(lines, NewStyle().YellowForeground().Render("⣿ Loading remote files..."))
		if len(lines) > height {
			lines = lines[:height]
		}
		return strings.Join(lines, "\n")
	}

	// Calculate how many file lines fit (height minus path line)
	fileLines := height - 1
	if fileLines < 1 {
		fileLines = 1
	}

	// File listing with scrolling
	if sectionIdx == 1 && len(m.localFiles) > 0 {
		scrollOffset := m.localScroll
		endIdx := scrollOffset + fileLines
		if endIdx > len(m.localFiles) {
			endIdx = len(m.localFiles)
		}

		for i := scrollOffset; i < endIdx; i++ {
			file := m.localFiles[i]
			if i == selected && isFocused {
				line := m.renderFileItemWide(file, true, width)
				lines = append(lines, NewStyle().GreenBg().Bold().Render(line))
			} else {
				line := m.renderFileItemWide(file, false, width)
				lines = append(lines, NewStyle().CyanForeground().Render(line))
			}
		}

		// Scroll indicator replaces last line if needed
		if len(m.localFiles) > fileLines {
			scrollIndicator := fmt.Sprintf(" ↕ %d/%d", selected+1, len(m.localFiles))
			// Replace last file line with indicator
			if len(lines) > height {
				lines = lines[:height-1]
			}
			lines = append(lines, NewStyle().YellowForeground().Render(scrollIndicator))
		}
	} else if sectionIdx == 2 && len(m.remoteFiles) > 0 {
		scrollOffset := m.remoteScroll
		endIdx := scrollOffset + fileLines
		if endIdx > len(m.remoteFiles) {
			endIdx = len(m.remoteFiles)
		}

		for i := scrollOffset; i < endIdx; i++ {
			file := m.remoteFiles[i]
			if i == selected && isFocused {
				line := m.renderRemoteItemWide(file, true, width)
				lines = append(lines, NewStyle().GreenBg().Bold().Render(line))
			} else {
				line := m.renderRemoteItemWide(file, false, width)
				lines = append(lines, NewStyle().CyanForeground().Render(line))
			}
		}

		// Scroll indicator
		if len(m.remoteFiles) > fileLines {
			scrollIndicator := fmt.Sprintf(" ↕ %d/%d", selected+1, len(m.remoteFiles))
			if len(lines) > height {
				lines = lines[:height-1]
			}
			lines = append(lines, NewStyle().YellowForeground().Render(scrollIndicator))
		}
	} else {
		var fileCount int
		if sectionIdx == 1 {
			fileCount = len(m.localFiles)
		} else {
			fileCount = len(m.remoteFiles)
		}
		if fileCount == 0 {
			lines = append(lines, NewStyle().CyanForeground().Render("(empty)"))
		}
	}

	// Final clamp
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

// renderConsolePanelContent renders the bottom console log panel (no border/title)
func (m Model) renderConsolePanelContent(width, height int) string {
	var lines []string

	if len(m.consoleLines) == 0 {
		lines = append(lines, NewStyle().CyanForeground().Render("(no activity yet)"))
	} else {
		visibleLines := height
		if visibleLines < 1 {
			visibleLines = 1
		}

		// Use consoleScroll if set, otherwise auto-scroll to bottom
		startIdx := len(m.consoleLines) - visibleLines
		if startIdx < 0 {
			startIdx = 0
		}
		// Manual scroll override
		if m.consoleScroll > 0 && m.consoleScroll < len(m.consoleLines) {
			startIdx = m.consoleScroll
		}

		endIdx := startIdx + visibleLines
		if endIdx > len(m.consoleLines) {
			endIdx = len(m.consoleLines)
		}

		for i := startIdx; i < endIdx; i++ {
			line := m.consoleLines[i]
			// Truncate long lines to panel width
			if len(line) > width {
				line = line[:width-3] + "..."
			}

			// Color based on content
			if strings.Contains(line, "FAILED") || strings.Contains(line, "Error") {
				lines = append(lines, NewStyle().RedForeground().Render(line))
			} else if strings.Contains(line, "Completed") {
				lines = append(lines, NewStyle().GreenForeground().Render(line))
			} else if strings.Contains(line, "Started") {
				lines = append(lines, NewStyle().YellowForeground().Render(line))
			} else {
				lines = append(lines, NewStyle().CyanForeground().Render(line))
			}
		}
	}

	// Clamp
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

// renderFileItemWide renders a file entry with proper spacing for wide display.
// Uses lipgloss.Width() for ANSI/emoji-safe display width measurement.
func (m Model) renderFileItemWide(file *commands.FileEntry, isSelected bool, availWidth int) string {
	prefix := "  "
	if isSelected {
		prefix = "▶ "
	}

	icon := "📄 "
	if file.IsDir {
		icon = "📁 "
	}

	sizeStr := formatFileSize(file.Size)

	// Use display width (not byte length) for emojis and special chars
	usedWidth := lipgloss.Width(prefix) + lipgloss.Width(icon) + len(sizeStr) + 1
	nameWidth := availWidth - usedWidth
	if nameWidth < 8 {
		nameWidth = 8
	}

	name := file.Name
	if len(name) > nameWidth {
		name = name[:nameWidth-1] + "…"
	}

	return fmt.Sprintf("%s%s%-*s %s", prefix, icon, nameWidth, name, sizeStr)
}

// renderRemoteItemWide renders a remote file entry with proper spacing.
func (m Model) renderRemoteItemWide(file *commands.RemoteEntry, isSelected bool, availWidth int) string {
	prefix := "  "
	if isSelected {
		prefix = "▶ "
	}

	icon := "📄 "
	if file.IsDir {
		icon = "📁 "
	}

	sizeStr := formatFileSize(file.Size)

	usedWidth := lipgloss.Width(prefix) + lipgloss.Width(icon) + len(sizeStr) + 1
	nameWidth := availWidth - usedWidth
	if nameWidth < 8 {
		nameWidth = 8
	}

	name := file.Name
	if len(name) > nameWidth {
		name = name[:nameWidth-1] + "…"
	}

	return fmt.Sprintf("%s%s%-*s %s", prefix, icon, nameWidth, name, sizeStr)
}
