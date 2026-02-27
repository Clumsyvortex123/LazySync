package commands

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// SCPCommand wraps OSCommand to provide SCP file transfer operations.
type SCPCommand struct {
	osCmd *OSCommand
	log   *logrus.Entry
}

// NewSCPCommand creates a new SCPCommand instance.
func NewSCPCommand(osCmd *OSCommand, log *logrus.Entry) *SCPCommand {
	return &SCPCommand{
		osCmd: osCmd,
		log:   log,
	}
}

// ParseSCPArgs builds the argument list for an scp invocation.
func ParseSCPArgs(host *SSHHost, srcFiles []string, destPath string, recursive bool) []string {
	var args []string

	// Identity file
	if host.KeyPath != "" {
		args = append(args, "-i", host.KeyPath)
	}

	// Port (default to 22)
	port := host.Port
	if port == 0 {
		port = 22
	}
	args = append(args, "-P", fmt.Sprintf("%d", port))

	// Recursive flag
	if recursive {
		args = append(args, "-r")
	}

	// Source files
	args = append(args, srcFiles...)

	// Destination: user@hostname:destPath
	dest := fmt.Sprintf("%s@%s:%s", host.User, host.Hostname, destPath)
	args = append(args, dest)

	return args
}

// ExecuteSCP runs an scp command to transfer files to a remote host.
func (s *SCPCommand) ExecuteSCP(ctx context.Context, host *SSHHost, srcFiles []string, destPath string, recursive bool, progressWriter io.Writer) error {
	args := ParseSCPArgs(host, srcFiles, destPath, recursive)

	s.log.WithField("args", strings.Join(args, " ")).Info("executing scp")

	cmd := exec.CommandContext(ctx, "scp", args...)

	return s.osCmd.RunCommandWithStreaming(cmd, progressWriter)
}
