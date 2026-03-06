package presentation

import (
	"fmt"

	"lazysync/pkg/commands"
)

// GetFileDisplayStrings formats file info for display
func GetFileDisplayStrings(file *commands.FileEntry) []string {
	icon := "📄"
	if file.IsDir {
		icon = "📁"
	}

	mark := " "
	if file.Marked {
		mark = "✓"
	}

	sizeStr := fmt.Sprintf("%d B", file.Size)
	if file.Size > 1024 {
		sizeStr = fmt.Sprintf("%d KB", file.Size/1024)
	}
	if file.Size > 1024*1024 {
		sizeStr = fmt.Sprintf("%d MB", file.Size/(1024*1024))
	}

	return []string{
		mark,
		icon,
		file.Name,
		sizeStr,
	}
}
