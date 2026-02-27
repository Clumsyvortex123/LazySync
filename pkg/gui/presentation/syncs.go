package presentation

import (
	"fmt"

	"lazyscpsync/pkg/commands"
)

// GetSyncDisplayStrings formats sync session info for display
func GetSyncDisplayStrings(session *commands.SyncSession) []string {
	status := session.Status.String()
	direction := fmt.Sprintf("%s → %s:%s", session.LocalPath, session.Host.Name, session.RemotePath)

	lastSync := "Never"
	if !session.LastSyncAt.IsZero() {
		lastSync = session.LastSyncAt.Format("15:04:05")
	}

	return []string{
		session.Host.Name,
		direction,
		status,
		lastSync,
	}
}
