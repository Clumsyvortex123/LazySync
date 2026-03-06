package gui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"lazyscpsync/pkg/commands"
)

// humanSize formats a byte count into a human-readable string (B, KB, MB, GB).
func humanSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.1fGB", float64(size)/(1024*1024*1024))
}

// renderMainView renders the main TUI layout fitted exactly to terminal size using lipgloss.
func (m Model) renderMainView() string {
	W := m.width
	H := m.height

	// Header and footer each take 1 line
	headerStyle := lipgloss.NewStyle().
		Width(W).
		Foreground(lipgloss.Color(ColorMagenta)).
		Bold(true)
	header := headerStyle.Render(" ⚡ LAZYSCPSYNC")

	footerStyle := lipgloss.NewStyle().
		Width(W).
		Foreground(lipgloss.Color(ColorCyan))
	footer := footerStyle.Render(" Tab:Switch ↑↓:Nav ←→:Dir a:Add f:Fetch o:SSH s:SCP l:Sync z:Procs ?:Help q:Quit")

	// Body fills remaining height
	bodyH := H - 2
	if bodyH < 6 {
		bodyH = 6
	}

	// Row heights (outer including borders)
	topH := bodyH / 4
	consoleH := bodyH / 5
	if topH < 4 {
		topH = 4
	}
	if consoleH < 3 {
		consoleH = 3
	}
	midH := bodyH - topH - consoleH
	if midH < 4 {
		midH = 4
	}

	// Column widths (outer including borders)
	leftW := W / 2
	rightW := W - leftW

	// Content dimensions: outer - 2 (border) - 1 (title line inside panel)
	cw := func(outerW int) int {
		if outerW-2 < 1 {
			return 1
		}
		return outerW - 2
	}
	ch := func(outerH int) int { // -2 border, -1 title
		if outerH-3 < 1 {
			return 1
		}
		return outerH - 3
	}

	// Build each panel using lipgloss bordered containers
	hostPanel := m.renderBorderedPanel(
		"[1] SSH HOSTS", leftW, topH,
		m.renderHostsPanelContent(cw(leftW), ch(topH)),
		m.focusedSection == 0,
	)
	statusPanel := m.renderBorderedPanel(
		"[2] STATUS", rightW, topH,
		m.renderStatusPanelContent(cw(rightW), ch(topH)),
		false,
	)

	localPanel := m.renderBorderedPanel(
		"[3] LOCAL FILES", leftW, midH,
		m.renderFileBrowserPanelContent(1, cw(leftW), ch(midH), "📂"),
		m.focusedSection == 1,
	)
	remotePanel := m.renderBorderedPanel(
		"[4] REMOTE FILES", rightW, midH,
		m.renderFileBrowserPanelContent(2, cw(rightW), ch(midH), "📡"),
		m.focusedSection == 2,
	)

	consolePanel := m.renderBorderedPanel(
		"[5] CONSOLE", W, consoleH,
		m.renderConsolePanelContent(cw(W), ch(consoleH)),
		m.focusedSection == 5,
	)

	// Compose layout with lipgloss
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, hostPanel, statusPanel)
	midRow := lipgloss.JoinHorizontal(lipgloss.Top, localPanel, remotePanel)
	body := lipgloss.JoinVertical(lipgloss.Left, topRow, midRow, consolePanel)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// renderBorderedPanel renders a lipgloss bordered panel with exact outer dimensions.
//
// Dimension model (lipgloss v0.11.1):
//   - Width(x)  → sets CONTENT width to x; border adds 2 to outer
//   - Height(x) → pads content to x lines; does NOT truncate
//   - MaxWidth(x) / MaxHeight(x) → truncates the FINAL output (AFTER borders)
//     So MaxWidth must be outerW, not contentW, to avoid clipping border chars.
//
// Strategy: pre-truncate content to contentH lines ourselves, then let
// lipgloss handle width (via Width) and border drawing. Use MaxWidth/MaxHeight
// on OUTER dimensions as safety nets only.
func (m Model) renderBorderedPanel(title string, outerW, outerH int, content string, isFocused bool) string {
	borderColor := lipgloss.Color(ColorCyan)
	if isFocused {
		borderColor = lipgloss.Color(ColorGreen)
	}

	// Title line styled
	titleStyle := lipgloss.NewStyle().Bold(true)
	if isFocused {
		titleStyle = titleStyle.Background(lipgloss.Color(ColorGreen)).Foreground(lipgloss.Color(ColorBlack))
	} else {
		titleStyle = titleStyle.Foreground(lipgloss.Color(ColorCyan))
	}
	header := titleStyle.Render(title)

	// Content dimensions = outer minus border (1 char each side for RoundedBorder)
	contentW := outerW - 2
	contentH := outerH - 2
	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}

	// Pre-truncate to exactly contentH lines (title + body).
	// lipgloss Height() only pads, it does NOT clip.
	allLines := strings.Split(header+"\n"+content, "\n")
	if len(allLines) > contentH {
		allLines = allLines[:contentH]
	}
	for len(allLines) < contentH {
		allLines = append(allLines, "")
	}
	inner := strings.Join(allLines, "\n")

	return lipgloss.NewStyle().
		Width(contentW).                    // content width (border adds 2 → outerW)
		MaxWidth(outerW).                   // safety: clip OUTER width (preserves border)
		MaxHeight(outerH).                  // safety: clip OUTER height (preserves border)
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Render(inner)
}

// renderStatusPanelContent renders the status/info panel
func (m Model) renderStatusPanelContent(width, height int) string {
	var lines []string

	// Show selected host info
	if len(m.sections[0].Items) > 0 && m.selectedInSection[0] < len(m.sections[0].Items) {
		selectedHost := m.sections[0].Items[m.selectedInSection[0]]
		lines = append(lines, NewStyle().CyanForeground().Render("Host: ")+NewStyle().GreenForeground().Bold().Render(selectedHost.Label))
	} else {
		lines = append(lines, NewStyle().CyanForeground().Render("No host selected"))
	}

	lines = append(lines, "")

	// Show active SCP/sync processes
	activeCount := 0
	for _, proc := range m.activeProcesses {
		if proc.Status == "running" || proc.Status == "watching" {
			activeCount++
		}
	}

	if activeCount > 0 {
		lines = append(lines, NewStyle().YellowForeground().Bold().Render(fmt.Sprintf("Transfers: %d active", activeCount)))
		for _, proc := range m.activeProcesses {
			elapsed := time.Since(proc.StartTime).Truncate(time.Second)
			var icon, style string
			switch proc.Status {
			case "running":
				icon = "⣿"
				style = "yellow"
			case "watching":
				icon = "⟳"
				style = "magenta"
			case "completed":
				icon = "✓"
				style = "green"
			case "stopped":
				icon = "■"
				style = "cyan"
			case "error":
				icon = "✗"
				style = "red"
			}

			line := fmt.Sprintf(" %s %s %s", icon, proc.ID, elapsed)
			switch style {
			case "yellow":
				lines = append(lines, NewStyle().YellowForeground().Render(line))
			case "magenta":
				lines = append(lines, NewStyle().MagentaForeground().Render(line))
			case "green":
				lines = append(lines, NewStyle().GreenForeground().Render(line))
			case "cyan":
				lines = append(lines, NewStyle().CyanForeground().Render(line))
			case "red":
				lines = append(lines, NewStyle().RedForeground().Render(line))
			}
		}
	} else if len(m.activeProcesses) > 0 {
		// Some finished processes waiting for cleanup
		lines = append(lines, NewStyle().GreenForeground().Render("Transfers: all done"))
		for _, proc := range m.activeProcesses {
			icon := "✓"
			switch proc.Status {
			case "error":
				icon = "✗"
			case "stopped":
				icon = "■"
			}
			lines = append(lines, NewStyle().CyanForeground().Render(fmt.Sprintf(" %s %s", icon, proc.ID)))
		}
	} else {
		lines = append(lines, NewStyle().CyanForeground().Render("Transfers: idle"))
	}

	// Show sync status
	lines = append(lines, "")
	if len(m.sections[3].Items) > 0 {
		lines = append(lines, NewStyle().MagentaForeground().Bold().Render(fmt.Sprintf("Syncs: %d", len(m.sections[3].Items))))
		for _, sync := range m.sections[3].Items {
			lines = append(lines, NewStyle().GreenForeground().Render(" ✓ "+sync.Label))
		}
	} else {
		lines = append(lines, NewStyle().CyanForeground().Render("Syncs: none"))
	}

	// Clamp to available height
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

// renderDialog renders modal dialogs overlaid on main view
func (m Model) renderDialog() string {
	mainView := m.renderMainView()

	switch m.dialogState {
	case DialogAddHost:
		return m.renderAddHostDialogOverlay(mainView)
	case DialogConfirmDelete:
		return m.renderConfirmDialogOverlay(mainView)
	case DialogSCPConfirm:
		return m.renderSCPConfirmDialogOverlay(mainView)
	case DialogSCPSelectSource:
		return m.renderSCPSelectSourceDialogOverlay(mainView)
	case DialogSCPSelectDest:
		return m.renderSCPSelectDestDialogOverlay(mainView)
	case DialogSCPSelectSourceFiles:
		return m.renderSCPSelectSourceFilesDialogOverlay(mainView)
	case DialogSCPSelectDestPath:
		return m.renderSCPSelectDestPathDialogOverlay(mainView)
	case DialogSCPConfirmCommand:
		return m.renderSCPConfirmCommandDialogOverlay(mainView)
	case DialogSCPExecuting:
		return m.renderSCPExecutingDialogOverlay(mainView)
	case DialogSCPActiveProcesses:
		return m.renderSCPActiveProcessesDialogOverlay(mainView)
	case DialogSyncConfirm:
		return m.renderSyncConfirmDialogOverlay(mainView)
	case DialogSyncSelectLocalPath:
		return m.renderSyncSelectLocalPathDialogOverlay(mainView)
	case DialogSyncSelectRemotePath:
		return m.renderSyncSelectRemotePathDialogOverlay(mainView)
	case DialogSyncOptions:
		return m.renderSyncOptionsDialogOverlay(mainView)
	case DialogSyncConfirmCommand:
		return m.renderSyncConfirmCommandDialogOverlay(mainView)
	case DialogCreateFolder:
		return m.renderCreateFolderDialogOverlay(mainView)
	case DialogHelp:
		return m.renderHelpDialogOverlay(mainView)
	default:
		return mainView
	}
}

// renderAddHostDialogOverlay renders the add host dialog as an overlay
func (m Model) renderAddHostDialogOverlay(mainView string) string {
	fieldNames := []string{"Name", "Hostname", "User", "Port", "Key Path"}
	fieldKeys := []string{"name", "hostname", "user", "port", "keypath"}

	var formLines []string
	title := NewStyle().
		MagentaForeground().
		Bold().
		Padding(0, 1).
		Render("Add New SSH Host")
	formLines = append(formLines, title)
	formLines = append(formLines, "")

	// Render form fields
	for i, fieldName := range fieldNames {
		fieldKey := fieldKeys[i]
		value := m.dialogFields[fieldKey]
		isFocused := m.dialogFocus == i

		fieldStyle := NewStyle().CyanForeground()
		if isFocused {
			fieldStyle = NewStyle().GreenBg().Bold()
		}

		// Add cursor to focused field
		display := value
		if isFocused {
			display = value + "█"
		}

		line := fieldStyle.
			Padding(0, 1).
			Render(fmt.Sprintf("%s: %s", fieldName, display))
		formLines = append(formLines, line)
	}

	formLines = append(formLines, "")
	formLines = append(formLines, NewStyle().GreenForeground().Render("Enter: Save | Esc: Cancel | Tab: Next Field"))

	form := strings.Join(formLines, "\n")

	dialog := NewStyle().
		Width(60).
		Height(15).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorCyan).
		Padding(1, 1).
		Render(form)

	// Center the dialog on screen
	dialogOverlay := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
	)

	return dialogOverlay
}

// renderConfirmDialogOverlay renders the confirmation dialog as an overlay
func (m Model) renderConfirmDialogOverlay(mainView string) string {
	if m.focusedSection >= len(m.sections) || m.selectedInSection[m.focusedSection] >= len(m.sections[m.focusedSection].Items) {
		return mainView
	}

	item := m.sections[m.focusedSection].Items[m.selectedInSection[m.focusedSection]]

	dialog := strings.Join([]string{
		NewStyle().RedForeground().Bold().Render("Delete Confirmation"),
		"",
		"Are you sure you want to delete?",
		NewStyle().YellowForeground().Bold().Render(item.Label),
		"",
		NewStyle().CyanForeground().Render("y: Yes | n: No | Esc: Cancel"),
	}, "\n")

	box := NewStyle().
		Width(50).
		Height(10).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorRed).
		Padding(1, 1).
		Render(dialog)

	dialogOverlay := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		box,
	)

	return dialogOverlay
}

// SCP Dialog Renderers

// renderSCPConfirmDialogOverlay renders the initial SCP confirmation dialog
func (m Model) renderSCPConfirmDialogOverlay(mainView string) string {
	dialog := strings.Join([]string{
		NewStyle().CyanForeground().Bold().Render("SCP File Transfer"),
		"",
		"Do you want to start a file transfer?",
		"",
		NewStyle().YellowForeground().Render("Enter: Continue | Esc: Cancel"),
	}, "\n")

	box := NewStyle().
		Width(50).
		Height(10).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorCyan).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderSCPSelectSourceDialogOverlay renders source system selection
func (m Model) renderSCPSelectSourceDialogOverlay(mainView string) string {
	localMark := " "
	remoteMark := " "
	if m.scpSourceIsLocal {
		localMark = "▶"
	} else {
		remoteMark = "▶"
	}

	dialog := strings.Join([]string{
		NewStyle().CyanForeground().Bold().Render("Select Source"),
		"",
		NewStyle().GreenForeground().Render(localMark + " Local"),
		NewStyle().YellowForeground().Render(remoteMark + " Remote SSH"),
		"",
		NewStyle().CyanForeground().Render("↑↓: Navigate | Enter: Continue | b: Back | Esc: Cancel"),
	}, "\n")

	box := NewStyle().
		Width(50).
		Height(12).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorCyan).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderSCPSelectDestDialogOverlay renders destination system selection
func (m Model) renderSCPSelectDestDialogOverlay(mainView string) string {
	localMark := " "
	remoteMark := " "
	if m.scpSourceIsLocal {
		// If source is local, destination is remote
		remoteMark = "▶"
	} else {
		// If source is remote, destination is local
		localMark = "▶"
	}

	dialog := strings.Join([]string{
		NewStyle().CyanForeground().Bold().Render("Select Destination"),
		"",
		NewStyle().GreenForeground().Render(localMark + " Local"),
		NewStyle().YellowForeground().Render(remoteMark + " Remote SSH"),
		"",
		NewStyle().CyanForeground().Render("↑↓: Navigate | Enter: Continue | b: Back | Esc: Cancel"),
	}, "\n")

	box := NewStyle().
		Width(50).
		Height(12).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorCyan).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderSCPSelectSourceFilesDialogOverlay renders file selection from source
func (m Model) renderSCPSelectSourceFilesDialogOverlay(mainView string) string {
	var lines []string
	lines = append(lines, NewStyle().CyanForeground().Bold().Render("Select Files to Transfer"))
	lines = append(lines, "")

	// Live SCP command preview at the top — updates as files are marked
	cmdPreview := m.buildPartialSCPCommand()
	cmdStyle := NewStyle().MagentaForeground()
	lines = append(lines, cmdStyle.Render("$ "+cmdPreview))
	lines = append(lines, NewStyle().CyanForeground().Render(strings.Repeat("─", 68)))

	// Determine which files to display
	var files interface{}
	var selectedIdx int
	var scrollOffset int
	var currentPath string

	if m.scpSourceIsLocal {
		files = m.localFiles
		selectedIdx = m.selectedInSection[1]
		scrollOffset = m.localScroll
		currentPath = m.localPath
	} else {
		files = m.remoteFiles
		selectedIdx = m.selectedInSection[2]
		scrollOffset = m.remoteScroll
		currentPath = m.remotePath
	}

	// Show path and marked files count
	lines = append(lines, NewStyle().CyanForeground().Render("📁 "+currentPath))
	markedCount := 0
	for _, marked := range m.scpMarkedFilePaths {
		if marked {
			markedCount++
		}
	}
	lines = append(lines, NewStyle().YellowForeground().Render(fmt.Sprintf("Marked: %d files", markedCount)))
	lines = append(lines, "")

	// Show file list with marking and scrolling
	visibleHeight := 12

	if localList, ok := files.([]*commands.FileEntry); ok {
		if len(localList) == 0 {
			lines = append(lines, NewStyle().CyanForeground().Render("(empty directory)"))
		} else {
			endIdx := scrollOffset + visibleHeight
			if endIdx > len(localList) {
				endIdx = len(localList)
			}

			for i := scrollOffset; i < endIdx; i++ {
			file := localList[i]
			mark := "  "
			if m.scpMarkedFilePaths[file.Path] {
				mark = "✓ "
			}
			icon := "📄"
			sizeStr := humanSize(file.Size)
			if file.IsDir {
				icon = "📁"
				sizeStr = ""
			}

			if i == selectedIdx {
				line := fmt.Sprintf("▶ %s %s%-30s %s", mark, icon, file.Name, sizeStr)
				lines = append(lines, NewStyle().GreenBg().Bold().Render(line))
			} else {
				line := fmt.Sprintf("  %s %s%-30s %s", mark, icon, file.Name, sizeStr)
				lines = append(lines, NewStyle().CyanForeground().Render(line))
			}
			}
		}
	} else if remoteList, ok := files.([]*commands.RemoteEntry); ok {
		if len(remoteList) == 0 {
			lines = append(lines, NewStyle().CyanForeground().Render("(empty directory)"))
		} else {
			endIdx := scrollOffset + visibleHeight
			if endIdx > len(remoteList) {
				endIdx = len(remoteList)
			}

			for i := scrollOffset; i < endIdx; i++ {
			file := remoteList[i]
			mark := "  "
			if m.scpMarkedFilePaths[file.Path] {
				mark = "✓ "
			}
			icon := "📄"
			sizeStr := humanSize(file.Size)
			if file.IsDir {
				icon = "📁"
				sizeStr = ""
			}

			if i == selectedIdx {
				line := fmt.Sprintf("▶ %s %s%-30s %s", mark, icon, file.Name, sizeStr)
				lines = append(lines, NewStyle().GreenBg().Bold().Render(line))
			} else {
				line := fmt.Sprintf("  %s %s%-30s %s", mark, icon, file.Name, sizeStr)
				lines = append(lines, NewStyle().CyanForeground().Render(line))
			}
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, NewStyle().YellowForeground().Render("←→: Dir | t: Mark | ↑↓: Nav | Enter: Confirm | b: Back | Esc: Cancel"))

	dialog := strings.Join(lines, "\n")

	box := NewStyle().
		Width(75).
		Height(22).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorCyan).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderSCPSelectDestPathDialogOverlay renders destination path selection
func (m Model) renderSCPSelectDestPathDialogOverlay(mainView string) string {
	var lines []string
	lines = append(lines, NewStyle().CyanForeground().Bold().Render("Select Destination Path"))
	lines = append(lines, "")

	// Live SCP command preview — dest path updates as user navigates
	cmdPreview := m.buildPartialSCPCommand()
	cmdStyle := NewStyle().MagentaForeground()
	lines = append(lines, cmdStyle.Render("$ "+cmdPreview))
	lines = append(lines, NewStyle().CyanForeground().Render(strings.Repeat("─", 68)))

	// Show current path
	var currentPath string
	var files interface{}
	var selectedIdx int
	var scrollOffset int

	if m.scpSourceIsLocal {
		// Source is local, destination is remote
		currentPath = m.remotePath
		files = m.remoteFiles
		selectedIdx = m.selectedInSection[2]
		scrollOffset = m.remoteScroll
	} else {
		// Source is remote, destination is local
		currentPath = m.localPath
		files = m.localFiles
		selectedIdx = m.selectedInSection[1]
		scrollOffset = m.localScroll
	}

	lines = append(lines, NewStyle().CyanForeground().Render("📁 "+currentPath))
	lines = append(lines, "")

	// Show file/folder list for destination navigation with scrolling
	visibleHeight := 12

	if localList, ok := files.([]*commands.FileEntry); ok {
		endIdx := scrollOffset + visibleHeight
		if endIdx > len(localList) {
			endIdx = len(localList)
		}

		for i := scrollOffset; i < endIdx; i++ {
			file := localList[i]
			icon := "📄"
			sizeStr := humanSize(file.Size)
			if file.IsDir {
				icon = "📁"
				sizeStr = ""
			}

			if i == selectedIdx {
				line := fmt.Sprintf("▶ %s %-30s %s", icon, file.Name, sizeStr)
				lines = append(lines, NewStyle().GreenBg().Bold().Render(line))
			} else {
				line := fmt.Sprintf("  %s %-30s %s", icon, file.Name, sizeStr)
				lines = append(lines, NewStyle().CyanForeground().Render(line))
			}
		}

		// Show scroll indicator
		if len(localList) > visibleHeight {
			lines = append(lines, NewStyle().CyanForeground().Render(fmt.Sprintf("↕ %d/%d", selectedIdx+1, len(localList))))
		}
	} else if remoteList, ok := files.([]*commands.RemoteEntry); ok {
		endIdx := scrollOffset + visibleHeight
		if endIdx > len(remoteList) {
			endIdx = len(remoteList)
		}

		for i := scrollOffset; i < endIdx; i++ {
			file := remoteList[i]
			icon := "📄"
			sizeStr := humanSize(file.Size)
			if file.IsDir {
				icon = "📁"
				sizeStr = ""
			}

			if i == selectedIdx {
				line := fmt.Sprintf("▶ %s %-30s %s", icon, file.Name, sizeStr)
				lines = append(lines, NewStyle().GreenBg().Bold().Render(line))
			} else {
				line := fmt.Sprintf("  %s %-30s %s", icon, file.Name, sizeStr)
				lines = append(lines, NewStyle().CyanForeground().Render(line))
			}
		}

		// Show scroll indicator
		if len(remoteList) > visibleHeight {
			lines = append(lines, NewStyle().CyanForeground().Render(fmt.Sprintf("↕ %d/%d", selectedIdx+1, len(remoteList))))
		}
	}

	lines = append(lines, "")
	lines = append(lines, NewStyle().YellowForeground().Render("←→: Dir | n: NewDir | ↑↓: Nav | Enter: Confirm | b: Back | Esc: Cancel"))

	dialog := strings.Join(lines, "\n")

	box := NewStyle().
		Width(75).
		Height(22).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorCyan).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderSCPConfirmCommandDialogOverlay renders the command confirmation
func (m Model) renderSCPConfirmCommandDialogOverlay(mainView string) string {
	cmd := m.scpExecCommand
	if cmd == "" {
		cmd = m.constructSCPCommand()
	}

	var lines []string
	lines = append(lines, NewStyle().GreenForeground().Bold().Render("✓ Ready to Execute"))
	lines = append(lines, "")

	// Source/dest summary
	if m.scpSourceIsLocal {
		lines = append(lines, NewStyle().CyanForeground().Render("  Local → Remote SSH"))
	} else {
		lines = append(lines, NewStyle().CyanForeground().Render("  Remote SSH → Local"))
	}
	lines = append(lines, NewStyle().CyanForeground().Render(fmt.Sprintf("  Files: %d", len(m.scpSelectedFilePaths))))
	lines = append(lines, "")

	// Final command with separator
	lines = append(lines, NewStyle().CyanForeground().Render(strings.Repeat("─", 68)))
	lines = append(lines, NewStyle().MagentaForeground().Bold().Render("$ "+cmd))
	lines = append(lines, NewStyle().CyanForeground().Render(strings.Repeat("─", 68)))

	lines = append(lines, "")
	lines = append(lines, NewStyle().YellowForeground().Bold().Render("Enter: Execute | b: Back | Esc: Cancel"))

	dialog := strings.Join(lines, "\n")

	box := NewStyle().
		Width(75).
		Height(16).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorGreen).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderSCPExecutingDialogOverlay renders the SCP execution status
func (m Model) renderSCPExecutingDialogOverlay(mainView string) string {
	var lines []string

	if m.scpProcessOutput == "" {
		// Still running
		lines = append(lines, NewStyle().YellowForeground().Bold().Render("⣿ Transferring..."))
		lines = append(lines, "")
		lines = append(lines, NewStyle().MagentaForeground().Render("$ "+m.scpExecCommand))
		lines = append(lines, "")
		lines = append(lines, NewStyle().CyanForeground().Render("Waiting for SCP to complete..."))
	} else {
		// Finished — show output
		if strings.Contains(m.scpProcessOutput, "Error:") || strings.Contains(m.scpProcessOutput, "Cancelled") {
			lines = append(lines, NewStyle().RedForeground().Bold().Render("✗ Transfer Failed"))
		} else {
			lines = append(lines, NewStyle().GreenForeground().Bold().Render("✓ Transfer Complete"))
		}
		lines = append(lines, "")
		lines = append(lines, NewStyle().MagentaForeground().Render("$ "+m.scpExecCommand))
		lines = append(lines, NewStyle().CyanForeground().Render(strings.Repeat("─", 68)))

		// Show output (truncate if very long)
		output := m.scpProcessOutput
		outputLines := strings.Split(output, "\n")
		maxLines := 8
		if len(outputLines) > maxLines {
			outputLines = outputLines[len(outputLines)-maxLines:]
		}
		for _, l := range outputLines {
			lines = append(lines, NewStyle().CyanForeground().Render(l))
		}
	}

	lines = append(lines, "")
	lines = append(lines, NewStyle().YellowForeground().Render("Esc: Close / Cancel"))

	dialog := strings.Join(lines, "\n")

	borderColor := ColorYellow
	if m.scpProcessOutput != "" {
		if strings.Contains(m.scpProcessOutput, "Error:") || strings.Contains(m.scpProcessOutput, "Cancelled") {
			borderColor = ColorRed
		} else {
			borderColor = ColorGreen
		}
	}

	box := NewStyle().
		Width(75).
		Height(18).
		Border(lipgloss.RoundedBorder()).
		BorderFg(borderColor).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderSCPActiveProcessesDialogOverlay renders the active processes as checkboxes
func (m Model) renderSCPActiveProcessesDialogOverlay(mainView string) string {
	var lines []string
	lines = append(lines, NewStyle().CyanForeground().Bold().Render("Active Processes"))
	lines = append(lines, "")

	if len(m.processSnapshot) == 0 {
		lines = append(lines, NewStyle().CyanForeground().Render("(No active processes)"))
	} else {
		for i, id := range m.processSnapshot {
			proc, exists := m.activeProcesses[id]
			if !exists {
				continue
			}

			// Checkbox
			check := "[ ]"
			if m.processMarked[id] {
				check = "[✓]"
			}

			// Cursor
			cursor := "  "
			if i == m.processListScroll {
				cursor = "▶ "
			}

			// Status icon
			var statusIcon string
			switch proc.Status {
			case "running":
				statusIcon = "🟡"
			case "watching":
				statusIcon = "🟢"
			default:
				statusIcon = "⚪"
			}

			elapsed := time.Since(proc.StartTime).Truncate(time.Second)
			line := fmt.Sprintf("%s%s %s %s %s→%s [%s] %s", cursor, check, statusIcon, proc.ID, proc.Source, proc.Dest, proc.Status, elapsed)

			if i == m.processListScroll {
				lines = append(lines, NewStyle().GreenBg().Bold().Render(line))
			} else if m.processMarked[id] {
				lines = append(lines, NewStyle().YellowForeground().Render(line))
			} else {
				lines = append(lines, NewStyle().CyanForeground().Render(line))
			}
		}
	}

	// Count marked
	markedCount := 0
	for _, marked := range m.processMarked {
		if marked {
			markedCount++
		}
	}

	lines = append(lines, "")
	if markedCount > 0 {
		lines = append(lines, NewStyle().RedForeground().Bold().Render(fmt.Sprintf("  %d selected — press . to kill", markedCount)))
	}
	lines = append(lines, "")
	lines = append(lines, NewStyle().YellowForeground().Render("↑↓: Nav | Space: Toggle | .: Kill Selected | z/Esc: Close"))

	dialog := strings.Join(lines, "\n")

	box := NewStyle().
		Width(75).
		Height(20).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorCyan).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// Sync Dialog Renderers

// renderSyncConfirmDialogOverlay renders the initial sync confirmation dialog
func (m Model) renderSyncConfirmDialogOverlay(mainView string) string {
	hostLabel := ""
	if m.scpSelectedHost != nil {
		hostLabel = fmt.Sprintf("%s@%s", m.scpSelectedHost.User, m.scpSelectedHost.Hostname)
	}

	dialog := strings.Join([]string{
		NewStyle().CyanForeground().Bold().Render("Live Sync"),
		"",
		"Start a live sync session?",
		"",
		NewStyle().GreenForeground().Render("Host: " + hostLabel),
		"",
		NewStyle().YellowForeground().Render("Enter: Continue | Esc: Cancel"),
	}, "\n")

	box := NewStyle().
		Width(50).
		Height(12).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorCyan).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderSyncSelectLocalPathDialogOverlay renders the local path selection for sync
func (m Model) renderSyncSelectLocalPathDialogOverlay(mainView string) string {
	var lines []string
	lines = append(lines, NewStyle().CyanForeground().Bold().Render("Select Local Source Path"))
	lines = append(lines, "")
	lines = append(lines, NewStyle().CyanForeground().Render("📁 "+m.localPath))
	lines = append(lines, "")

	visibleHeight := 12
	scrollOffset := m.localScroll
	endIdx := scrollOffset + visibleHeight
	if endIdx > len(m.localFiles) {
		endIdx = len(m.localFiles)
	}

	if len(m.localFiles) == 0 {
		lines = append(lines, NewStyle().CyanForeground().Render("(empty directory)"))
	} else {
		for i := scrollOffset; i < endIdx; i++ {
			file := m.localFiles[i]
			icon := "📄"
			sizeStr := humanSize(file.Size)
			if file.IsDir {
				icon = "📁"
				sizeStr = ""
			}
			if i == m.selectedInSection[1] {
				line := fmt.Sprintf("▶ %s %-30s %s", icon, file.Name, sizeStr)
				lines = append(lines, NewStyle().GreenBg().Bold().Render(line))
			} else {
				line := fmt.Sprintf("  %s %-30s %s", icon, file.Name, sizeStr)
				lines = append(lines, NewStyle().CyanForeground().Render(line))
			}
		}

		if len(m.localFiles) > visibleHeight {
			lines = append(lines, NewStyle().YellowForeground().Render(fmt.Sprintf("↕ %d/%d", m.selectedInSection[1]+1, len(m.localFiles))))
		}
	}

	lines = append(lines, "")
	lines = append(lines, NewStyle().YellowForeground().Render("←→: Dir | t: Select | ↑↓: Nav | Enter: Confirm | b: Back | Esc: Cancel"))

	dialog := strings.Join(lines, "\n")

	box := NewStyle().
		Width(75).
		Height(22).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorCyan).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderSyncSelectRemotePathDialogOverlay renders the remote path selection for sync
func (m Model) renderSyncSelectRemotePathDialogOverlay(mainView string) string {
	var lines []string
	lines = append(lines, NewStyle().CyanForeground().Bold().Render("Select Remote Destination Path"))
	lines = append(lines, "")

	// Show loading indicator
	if m.isLoadingRemote {
		lines = append(lines, NewStyle().CyanForeground().Render("📁 "+m.remotePath))
		lines = append(lines, NewStyle().YellowForeground().Render("⣿ Loading remote files..."))
	} else {
		lines = append(lines, NewStyle().CyanForeground().Render("📁 "+m.remotePath))
		lines = append(lines, "")

		visibleHeight := 12
		scrollOffset := m.remoteScroll
		endIdx := scrollOffset + visibleHeight
		if endIdx > len(m.remoteFiles) {
			endIdx = len(m.remoteFiles)
		}

		if len(m.remoteFiles) == 0 {
			lines = append(lines, NewStyle().CyanForeground().Render("(empty directory)"))
		} else {
			for i := scrollOffset; i < endIdx; i++ {
				file := m.remoteFiles[i]
				icon := "📄"
				sizeStr := humanSize(file.Size)
				if file.IsDir {
					icon = "📁"
					sizeStr = ""
				}
				if i == m.selectedInSection[2] {
					line := fmt.Sprintf("▶ %s %-30s %s", icon, file.Name, sizeStr)
					lines = append(lines, NewStyle().GreenBg().Bold().Render(line))
				} else {
					line := fmt.Sprintf("  %s %-30s %s", icon, file.Name, sizeStr)
					lines = append(lines, NewStyle().CyanForeground().Render(line))
				}
			}

			if len(m.remoteFiles) > visibleHeight {
				lines = append(lines, NewStyle().YellowForeground().Render(fmt.Sprintf("↕ %d/%d", m.selectedInSection[2]+1, len(m.remoteFiles))))
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, NewStyle().YellowForeground().Render("←→: Dir | t: Select | n: NewDir | ↑↓: Nav | Enter: Confirm | b: Back | Esc: Cancel"))

	dialog := strings.Join(lines, "\n")

	box := NewStyle().
		Width(75).
		Height(22).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorCyan).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderSyncOptionsDialogOverlay renders the sync options checkboxes
func (m Model) renderSyncOptionsDialogOverlay(mainView string) string {
	var lines []string
	lines = append(lines, NewStyle().CyanForeground().Bold().Render("Sync Options"))
	lines = append(lines, "")

	// no-watch option
	noWatchCheck := "[ ]"
	if m.syncNoWatch {
		noWatchCheck = "[✓]"
	}
	noWatchCursor := "  "
	if m.syncOptionsCursor == 0 {
		noWatchCursor = "▶ "
	}
	noWatchLine := fmt.Sprintf("%s%s no-watch", noWatchCursor, noWatchCheck)
	if m.syncOptionsCursor == 0 {
		lines = append(lines, NewStyle().GreenBg().Bold().Render(noWatchLine))
	} else {
		lines = append(lines, NewStyle().CyanForeground().Render(noWatchLine))
	}

	// standard-git-exclude option
	gitExcludeCheck := "[ ]"
	if m.syncGitExclude {
		gitExcludeCheck = "[✓]"
	}
	gitExcludeCursor := "  "
	if m.syncOptionsCursor == 1 {
		gitExcludeCursor = "▶ "
	}
	gitExcludeLine := fmt.Sprintf("%s%s standard-git-exclude", gitExcludeCursor, gitExcludeCheck)
	if m.syncOptionsCursor == 1 {
		lines = append(lines, NewStyle().GreenBg().Bold().Render(gitExcludeLine))
	} else {
		lines = append(lines, NewStyle().CyanForeground().Render(gitExcludeLine))
	}

	lines = append(lines, "")

	// Live command preview
	lines = append(lines, NewStyle().CyanForeground().Render(strings.Repeat("─", 46)))
	previewCmd := m.constructLiveSyncCommand()
	lines = append(lines, NewStyle().MagentaForeground().Render("$ "+previewCmd))
	lines = append(lines, NewStyle().CyanForeground().Render(strings.Repeat("─", 46)))

	lines = append(lines, "")
	lines = append(lines, NewStyle().YellowForeground().Render("↑↓: Nav  Space: Toggle  Enter: OK  b: Back  Esc: Cancel"))

	dialog := strings.Join(lines, "\n")

	box := NewStyle().
		Width(55).
		Height(16).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorCyan).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderSyncConfirmCommandDialogOverlay renders the final sync command confirmation
func (m Model) renderSyncConfirmCommandDialogOverlay(mainView string) string {
	cmd := m.syncExecCommand
	if cmd == "" {
		cmd = m.constructLiveSyncCommand()
	}

	var lines []string
	lines = append(lines, NewStyle().GreenForeground().Bold().Render("✓ Ready to Execute Sync"))
	lines = append(lines, "")

	// Summary
	lines = append(lines, NewStyle().CyanForeground().Render(fmt.Sprintf("  Local:  %s", m.syncLocalPath)))
	if m.scpSelectedHost != nil {
		lines = append(lines, NewStyle().CyanForeground().Render(fmt.Sprintf("  Remote: %s@%s:%s", m.scpSelectedHost.User, m.scpSelectedHost.Hostname, m.syncRemotePath)))
	}
	lines = append(lines, "")

	// Options summary
	optionsSummary := "  Options:"
	if m.syncNoWatch {
		optionsSummary += " no-watch"
	}
	if m.syncGitExclude {
		optionsSummary += " standard-git-exclude"
	}
	if !m.syncNoWatch && !m.syncGitExclude {
		optionsSummary += " (none)"
	}
	lines = append(lines, NewStyle().YellowForeground().Render(optionsSummary))
	lines = append(lines, "")

	// Final command
	lines = append(lines, NewStyle().CyanForeground().Render(strings.Repeat("─", 68)))
	lines = append(lines, NewStyle().MagentaForeground().Bold().Render("$ "+cmd))
	lines = append(lines, NewStyle().CyanForeground().Render(strings.Repeat("─", 68)))

	lines = append(lines, "")
	lines = append(lines, NewStyle().YellowForeground().Bold().Render("Enter: Execute | b: Back | Esc: Cancel"))

	dialog := strings.Join(lines, "\n")

	box := NewStyle().
		Width(75).
		Height(18).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorGreen).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderCreateFolderDialogOverlay renders the create folder name input
func (m Model) renderCreateFolderDialogOverlay(mainView string) string {
	location := "local"
	basePath := m.localPath
	if m.createFolderIsRemote {
		location = "remote"
		basePath = m.remotePath
	}

	dialog := strings.Join([]string{
		NewStyle().CyanForeground().Bold().Render("Create New Folder (" + location + ")"),
		"",
		NewStyle().CyanForeground().Render("📁 " + basePath + "/"),
		"",
		NewStyle().GreenForeground().Bold().Render("Name: " + m.createFolderName + "█"),
		"",
		NewStyle().YellowForeground().Render("Enter: Create | Esc: Cancel"),
	}, "\n")

	box := NewStyle().
		Width(55).
		Height(12).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorCyan).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderHelpDialogOverlay renders the keybindings help popup
func (m Model) renderHelpDialogOverlay(mainView string) string {
	titleStyle := NewStyle().CyanForeground().Bold()
	sectionStyle := NewStyle().GreenForeground().Bold()
	keyStyle := NewStyle().YellowForeground()
	descStyle := NewStyle().CyanForeground()

	var lines []string
	lines = append(lines, titleStyle.Render("Keybindings Reference"))
	lines = append(lines, "")

	lines = append(lines, sectionStyle.Render("-- Global --"))
	for _, e := range [][2]string{
		{"Tab / Shift+Tab", "Cycle focus between panels"},
		{"?", "Show this help"},
		{"s", "Start SCP transfer dialog"},
		{"l", "Start Live Sync dialog"},
		{"z", "Show active processes"},
		{"q / Ctrl+C", "Quit"},
	} {
		lines = append(lines, fmt.Sprintf("  %s  %s", keyStyle.Render(fmt.Sprintf("%-18s", e[0])), descStyle.Render(e[1])))
	}
	lines = append(lines, "")

	lines = append(lines, sectionStyle.Render("-- SSH Hosts Panel --"))
	for _, e := range [][2]string{
		{"Up/Down or j/k", "Navigate hosts"},
		{"a", "Add new host"},
		{"d", "Delete selected host"},
		{"f", "Fetch remote files"},
		{"o", "Open SSH terminal"},
	} {
		lines = append(lines, fmt.Sprintf("  %s  %s", keyStyle.Render(fmt.Sprintf("%-18s", e[0])), descStyle.Render(e[1])))
	}
	lines = append(lines, "")

	lines = append(lines, sectionStyle.Render("-- File Browsers --"))
	for _, e := range [][2]string{
		{"Up/Down or j/k", "Navigate files"},
		{"Right / Enter / l", "Enter directory"},
		{"Left / Bksp / h", "Go to parent directory"},
	} {
		lines = append(lines, fmt.Sprintf("  %s  %s", keyStyle.Render(fmt.Sprintf("%-18s", e[0])), descStyle.Render(e[1])))
	}
	lines = append(lines, "")

	lines = append(lines, sectionStyle.Render("-- SCP Dialog --"))
	for _, e := range [][2]string{
		{"Up/Down", "Navigate / toggle selection"},
		{"Space", "Mark/unmark files"},
		{"Enter", "Confirm current step"},
		{"n", "Create new folder (dest)"},
		{"b", "Go back one step"},
		{"Esc", "Cancel"},
	} {
		lines = append(lines, fmt.Sprintf("  %s  %s", keyStyle.Render(fmt.Sprintf("%-18s", e[0])), descStyle.Render(e[1])))
	}
	lines = append(lines, "")

	lines = append(lines, sectionStyle.Render("-- Live Sync Dialog --"))
	for _, e := range [][2]string{
		{"Left/Right", "Navigate directories"},
		{"t", "Select highlighted folder"},
		{"Space", "Toggle options"},
		{"n", "Create new folder (remote)"},
		{"b", "Go back one step"},
		{"Esc", "Cancel"},
	} {
		lines = append(lines, fmt.Sprintf("  %s  %s", keyStyle.Render(fmt.Sprintf("%-18s", e[0])), descStyle.Render(e[1])))
	}
	lines = append(lines, "")

	lines = append(lines, sectionStyle.Render("-- Active Processes --"))
	for _, e := range [][2]string{
		{"Up/Down", "Navigate process list"},
		{"Space", "Mark/unmark process"},
		{".", "Kill all marked processes"},
		{"z / Esc", "Close"},
	} {
		lines = append(lines, fmt.Sprintf("  %s  %s", keyStyle.Render(fmt.Sprintf("%-18s", e[0])), descStyle.Render(e[1])))
	}

	lines = append(lines, "")
	lines = append(lines, NewStyle().YellowForeground().Render("Press Esc to close"))

	dialog := strings.Join(lines, "\n")

	box := NewStyle().
		Width(60).
		Height(len(lines) + 2).
		Border(lipgloss.RoundedBorder()).
		BorderFg(ColorCyan).
		Padding(1, 1).
		Render(dialog)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
