package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sasha-s/go-deadlock"
	"github.com/sirupsen/logrus"
)

// SyncStatus represents the state of a sync session
type SyncStatus int

const (
	SyncStatusIdle    SyncStatus = iota
	SyncStatusRunning
	SyncStatusError
	SyncStatusStopped
)

func (s SyncStatus) String() string {
	switch s {
	case SyncStatusIdle:
		return "Idle"
	case SyncStatusRunning:
		return "Running"
	case SyncStatusError:
		return "Error"
	case SyncStatusStopped:
		return "Stopped"
	default:
		return "Unknown"
	}
}

// SyncSession represents a single local-to-remote sync session
type SyncSession struct {
	ID         string
	LocalPath  string
	RemotePath string
	Host       *SSHHost
	Status     SyncStatus
	LastSyncAt time.Time
	cancel     context.CancelFunc
	mu         sync.Mutex
	lastError  string
}

// GetLastError returns the last error message for the session
func (s *SyncSession) GetLastError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError
}

// setStatus updates the session status (must be called with mu held or from RunSyncLoop)
func (s *SyncSession) setStatus(status SyncStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
}

// setError updates the session error and status
func (s *SyncSession) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = SyncStatusError
	s.lastError = err.Error()
}

// SyncManager manages all sync sessions
type SyncManager struct {
	sessions map[string]*SyncSession
	mu       deadlock.RWMutex
	osCmd    *OSCommand
	log      *logrus.Entry
}

// NewSyncManager creates a new SyncManager
func NewSyncManager(osCmd *OSCommand, log *logrus.Entry) *SyncManager {
	return &SyncManager{
		sessions: make(map[string]*SyncSession),
		osCmd:    osCmd,
		log:      log,
	}
}

// Add creates and registers a new sync session, returning its ID
func (m *SyncManager) Add(localPath, remotePath string, host *SSHHost) *SyncSession {
	id := fmt.Sprintf("sync-%d", time.Now().UnixNano())
	session := &SyncSession{
		ID:         id,
		LocalPath:  localPath,
		RemotePath: remotePath,
		Host:       host,
		Status:     SyncStatusIdle,
	}

	m.mu.Lock()
	m.sessions[id] = session
	m.mu.Unlock()

	return session
}

// Remove stops and removes a sync session by ID
func (m *SyncManager) Remove(id string) {
	m.mu.Lock()
	session, ok := m.sessions[id]
	if ok {
		if session.cancel != nil {
			session.cancel()
		}
		delete(m.sessions, id)
	}
	m.mu.Unlock()
}

// Get returns a sync session by ID
func (m *SyncManager) Get(id string) *SyncSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// List returns all sync sessions as a slice
func (m *SyncManager) List() []*SyncSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*SyncSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

// Close cancels all running sessions
func (m *SyncManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, session := range m.sessions {
		if session.cancel != nil {
			session.cancel()
		}
		session.setStatus(SyncStatusStopped)
	}
	return nil
}

// RunSyncLoop runs the sync loop for a session: initial rsync, then watch for changes
func RunSyncLoop(ctx context.Context, session *SyncSession, osCmd *OSCommand, log *logrus.Entry) {
	ctx, cancel := context.WithCancel(ctx)
	session.mu.Lock()
	session.cancel = cancel
	session.Status = SyncStatusRunning
	session.mu.Unlock()

	defer func() {
		cancel()
		session.setStatus(SyncStatusStopped)
	}()

	// Initial full rsync
	log.WithField("session", session.ID).Info("running initial rsync")
	if err := runRsync(session, osCmd); err != nil {
		log.WithError(err).Error("initial rsync failed")
		session.setError(err)
		return
	}
	session.mu.Lock()
	session.LastSyncAt = time.Now()
	session.mu.Unlock()

	// Set up file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.WithError(err).Error("failed to create file watcher")
		session.setError(err)
		return
	}
	defer watcher.Close()

	if err := watcher.Add(session.LocalPath); err != nil {
		log.WithError(err).Error("failed to watch directory")
		session.setError(err)
		return
	}

	// Debounce timer
	const debounceInterval = 500 * time.Millisecond
	var debounceTimer *time.Timer
	debounceCh := make(chan struct{}, 1)

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			log.WithField("event", event).Debug("file change detected")

			// Reset debounce timer
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceInterval, func() {
				select {
				case debounceCh <- struct{}{}:
				default:
				}
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.WithError(err).Warn("watcher error")

		case <-debounceCh:
			log.WithField("session", session.ID).Info("running debounced rsync")
			session.setStatus(SyncStatusRunning)
			if err := runRsync(session, osCmd); err != nil {
				log.WithError(err).Error("rsync failed")
				session.setError(err)
			} else {
				session.mu.Lock()
				session.LastSyncAt = time.Now()
				session.Status = SyncStatusRunning
				session.lastError = ""
				session.mu.Unlock()
			}
		}
	}
}

// runRsync executes the rsync command for a session
func runRsync(session *SyncSession, osCmd *OSCommand) error {
	args := buildRsyncCmd(session.Host, session.LocalPath, session.RemotePath)
	cmd := exec.Command(args[0], args[1:]...)
	output, err := osCmd.RunCommand(cmd)
	if err != nil {
		return fmt.Errorf("rsync failed: %s: %w", strings.TrimSpace(output), err)
	}
	return nil
}

// buildRsyncCmd returns the rsync command arguments for syncing local to remote
func buildRsyncCmd(host *SSHHost, localPath, remotePath string) []string {
	port := host.Port
	if port == 0 {
		port = 22
	}

	sshCmd := fmt.Sprintf("ssh -p %d", port)
	if host.KeyPath != "" {
		sshCmd = fmt.Sprintf("ssh -i %s -p %d", host.KeyPath, port)
	}

	rsyncPath := fmt.Sprintf("mkdir -p %s && rsync", remotePath)

	user := host.User
	if user == "" {
		user = "root"
	}

	src := localPath + "/"
	dst := fmt.Sprintf("%s@%s:%s/", user, host.Hostname, remotePath)

	return []string{
		"rsync",
		"--prune-empty-dirs",
		"--delete",
		"-a",
		"-z",
		"--checksum",
		"--no-t",
		"--exclude=.git/",
		"--exclude=__pycache__/",
		"--exclude=.DS_Store",
		"-e", sshCmd,
		"--rsync-path", rsyncPath,
		src,
		dst,
	}
}
