package commands

import (
	"io"
	"os/exec"
	"runtime"

	"github.com/sirupsen/logrus"
)

// OSCommand handles all shell command execution
type OSCommand struct {
	log *logrus.Entry
}

// NewOSCommand creates a new OSCommand instance
func NewOSCommand(log *logrus.Entry) *OSCommand {
	return &OSCommand{
		log: log,
	}
}

// RunCommand executes a command and returns stdout
func (o *OSCommand) RunCommand(cmd *exec.Cmd) (string, error) {
	output, err := cmd.CombinedOutput()
	if err != nil {
		o.log.WithError(err).WithField("cmd", cmd.String()).Error("command failed")
		return string(output), err
	}
	return string(output), nil
}

// RunCommandWithStreaming executes a command and streams output
func (o *OSCommand) RunCommandWithStreaming(cmd *exec.Cmd, writer io.Writer) error {
	cmd.Stdout = writer
	cmd.Stderr = writer
	err := cmd.Run()
	if err != nil {
		o.log.WithError(err).WithField("cmd", cmd.String()).Error("command failed")
	}
	return err
}

// GetPlatform returns the current OS
func (o *OSCommand) GetPlatform() string {
	return runtime.GOOS
}

// GetOS returns human-readable OS name
func (o *OSCommand) GetOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	default:
		return runtime.GOOS
	}
}
