package presentation

import (
	"fmt"

	"lazyscpsync/pkg/commands"
)

// GetHostDisplayStrings formats host info for display
func GetHostDisplayStrings(host *commands.SSHHost) []string {
	status := "●"
	if host.IsConnected {
		status = "✓"
	}

	syncStatus := ""
	if host.HasActiveSync {
		syncStatus = " [SYNCING]"
	}

	return []string{
		status,
		host.Name,
		fmt.Sprintf("%s@%s:%d%s", host.User, host.Hostname, host.Port, syncStatus),
	}
}
