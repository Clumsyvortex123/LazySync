package gui

import (
	"lazyscpsync/pkg/commands"
)

// Navigation messages
type SelectNextMsg struct{}
type SelectPrevMsg struct{}
type FocusNextSectionMsg struct{}
type FocusPrevSectionMsg struct{}

// Data loading messages
type HostsLoadedMsg []*commands.SSHHost
type FilesLoadedMsg []*commands.FileEntry
type RemoteFilesLoadedMsg struct {
	Path    string
	Entries []*commands.RemoteEntry
	Err     error
}
type SyncSessionsUpdatedMsg []*commands.SyncSession

// Action messages
type StartSCPMsg struct {
	LocalPath string
	RemotePath string
	Host *commands.SSHHost
}

type StartSyncMsg struct {
	LocalPath string
	RemotePath string
	Host *commands.SSHHost
}

type StopSyncMsg struct {
	SessionID string
}

// Async operation messages
type ErrorMsg string
type ProgressUpdateMsg float64

// SCP execution result
type SCPFinishedMsg struct {
	ProcessID string
	Output    string
	Err       error
}

// SCP background process started
type SCPStartedMsg struct {
	ProcessID string
}

// Request to clean up a completed process from status
type SCPCleanupMsg struct {
	ProcessID string
}

// Dialog messages
type OpenAddHostDialogMsg struct{}
type OpenDeleteDialogMsg struct{}
type CloseDialogMsg struct{}
type ClearErrorMsg struct{}

// Sync execution result
type SyncFinishedMsg struct {
	ProcessID string
	Output    string
	Err       error
}

// Host reachability check results
type HostReachabilityMsg struct {
	Results map[string]bool // host name → reachable
}

// Trigger to run the next reachability check after delay
type ReachabilityTickMsg struct{}

// Tick for periodic updates
type TickMsg struct{}

