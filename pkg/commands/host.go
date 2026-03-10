package commands

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jesseduffield/yaml"
	"github.com/sirupsen/logrus"

	"lazysync/pkg/config"
)

// SSHHost represents a single SSH host entry
type SSHHost struct {
	Name          string `yaml:"name"`
	Hostname      string `yaml:"hostname"`
	User          string `yaml:"user"`
	Port          int    `yaml:"port"`
	KeyPath       string `yaml:"key_path,omitempty"`
	IsConnected   bool   `yaml:"-"`
	HasActiveSync bool   `yaml:"-"`
}

// SSHHostCommand manages SSH host discovery and persistence
type SSHHostCommand struct {
	appConfig *config.AppConfig
	log       *logrus.Entry
	hosts     []*SSHHost
	mu        sync.RWMutex
}

// NewSSHHostCommand creates a new SSHHostCommand
func NewSSHHostCommand(appConfig *config.AppConfig, log *logrus.Entry) *SSHHostCommand {
	return &SSHHostCommand{
		appConfig: appConfig,
		log:       log,
	}
}

// LoadHosts reads hosts from ~/.ssh/config and the supplementary hosts.yml
func (c *SSHHostCommand) LoadHosts() ([]*SSHHost, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sshConfigPath := filepath.Join(os.Getenv("HOME"), ".ssh", "config")
	sshHosts, err := parseSSHConfig(sshConfigPath)
	if err != nil {
		c.log.WithError(err).Warn("failed to parse ssh config")
		sshHosts = nil
	}

	supplementaryHosts, err := loadSupplementaryHosts(c.appConfig.HostsFile)
	if err != nil {
		c.log.WithError(err).Warn("failed to load supplementary hosts")
		supplementaryHosts = nil
	}

	c.hosts = mergeHosts(sshHosts, supplementaryHosts)
	return c.hosts, nil
}

// AddHost adds a host to the supplementary hosts.yml file
func (c *SSHHostCommand) AddHost(host *SSHHost) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if host.Port == 0 {
		host.Port = 22
	}

	// Load existing supplementary hosts
	existing, err := loadSupplementaryHosts(c.appConfig.HostsFile)
	if err != nil {
		existing = nil
	}

	// Check for duplicate name
	for _, h := range existing {
		if h.Name == host.Name {
			return fmt.Errorf("host %q already exists in hosts.yml", host.Name)
		}
	}

	existing = append(existing, host)

	if err := saveSupplementaryHosts(c.appConfig.HostsFile, existing); err != nil {
		return err
	}

	// Update in-memory list
	c.hosts = mergeHosts(c.hosts, []*SSHHost{host})
	return nil
}

// UpdateHost updates an existing host in the supplementary hosts.yml file
func (c *SSHHostCommand) UpdateHost(originalName string, updated *SSHHost) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if updated.Port == 0 {
		updated.Port = 22
	}

	existing, err := loadSupplementaryHosts(c.appConfig.HostsFile)
	if err != nil {
		return fmt.Errorf("failed to load hosts file: %w", err)
	}

	found := false
	for i, h := range existing {
		if h.Name == originalName {
			existing[i] = updated
			found = true
			break
		}
	}

	if !found {
		// Host might be from ssh config — add it as supplementary
		existing = append(existing, updated)
	}

	if err := saveSupplementaryHosts(c.appConfig.HostsFile, existing); err != nil {
		return err
	}

	// Update in-memory list
	for i, h := range c.hosts {
		if h.Name == originalName {
			c.hosts[i] = updated
			return nil
		}
	}
	// If not found in memory, append
	c.hosts = append(c.hosts, updated)
	return nil
}

// SaveHostsToSSHConfig writes supplementary hosts to ~/.ssh/config
// (appends hosts that don't already exist in the file)
func (c *SSHHostCommand) SaveHostsToSSHConfig() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	supplementary, err := loadSupplementaryHosts(c.appConfig.HostsFile)
	if err != nil || len(supplementary) == 0 {
		return nil // nothing to save
	}

	sshConfigPath := filepath.Join(os.Getenv("HOME"), ".ssh", "config")

	// Parse existing ssh config to find which hosts already exist
	existingHosts, _ := parseSSHConfig(sshConfigPath)
	existingNames := make(map[string]bool)
	for _, h := range existingHosts {
		existingNames[h.Name] = true
	}

	// Collect hosts that need to be appended
	var toAppend []*SSHHost
	for _, h := range supplementary {
		if !existingNames[h.Name] {
			toAppend = append(toAppend, h)
		}
	}

	if len(toAppend) == 0 {
		return nil
	}

	// Ensure .ssh directory exists
	sshDir := filepath.Dir(sshConfigPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Open file for appending (create if not exists)
	f, err := os.OpenFile(sshConfigPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open ssh config: %w", err)
	}
	defer f.Close()

	for _, h := range toAppend {
		entry := fmt.Sprintf("\nHost %s\n    HostName %s\n    User %s\n    Port %d\n",
			h.Name, h.Hostname, h.User, h.Port)
		if h.KeyPath != "" {
			entry += fmt.Sprintf("    IdentityFile %s\n", h.KeyPath)
		}
		if _, err := f.WriteString(entry); err != nil {
			return fmt.Errorf("failed to write host %s: %w", h.Name, err)
		}
	}

	return nil
}

// RemoveHost removes a host from the supplementary hosts.yml file
func (c *SSHHostCommand) RemoveHost(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	existing, err := loadSupplementaryHosts(c.appConfig.HostsFile)
	if err != nil {
		return fmt.Errorf("failed to load hosts file: %w", err)
	}

	found := false
	filtered := make([]*SSHHost, 0, len(existing))
	for _, h := range existing {
		if h.Name == name {
			found = true
			continue
		}
		filtered = append(filtered, h)
	}

	if !found {
		return fmt.Errorf("host %q not found in hosts.yml", name)
	}

	if err := saveSupplementaryHosts(c.appConfig.HostsFile, filtered); err != nil {
		return err
	}

	// Remove from in-memory list
	newHosts := make([]*SSHHost, 0, len(c.hosts))
	for _, h := range c.hosts {
		if h.Name != name {
			newHosts = append(newHosts, h)
		}
	}
	c.hosts = newHosts
	return nil
}

// GetHost returns a host by name, or nil if not found
func (c *SSHHostCommand) GetHost(name string) *SSHHost {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, h := range c.hosts {
		if h.Name == name {
			return h
		}
	}
	return nil
}

// GetAllHosts returns all loaded hosts
func (c *SSHHostCommand) GetAllHosts() []*SSHHost {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*SSHHost, len(c.hosts))
	copy(result, c.hosts)
	return result
}

// parseSSHConfig parses ~/.ssh/config and extracts host entries
func parseSSHConfig(path string) ([]*SSHHost, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var hosts []*SSHHost
	var current *SSHHost

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split into keyword and arguments
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			// Try tab separator
			parts = strings.SplitN(line, "\t", 2)
			if len(parts) < 2 {
				continue
			}
		}

		keyword := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch keyword {
		case "host":
			// Finalize previous host
			if current != nil && current.Name != "" {
				if current.Port == 0 {
					current.Port = 22
				}
				hosts = append(hosts, current)
			}

			// Skip wildcard entries
			if strings.Contains(value, "*") || strings.Contains(value, "?") {
				current = nil
				continue
			}

			current = &SSHHost{
				Name: value,
			}

		case "hostname":
			if current != nil {
				current.Hostname = value
			}

		case "user":
			if current != nil {
				current.User = value
			}

		case "port":
			if current != nil {
				if p, err := strconv.Atoi(value); err == nil {
					current.Port = p
				}
			}

		case "identityfile":
			if current != nil {
				current.KeyPath = expandTilde(value)
			}
		}
	}

	// Don't forget the last host
	if current != nil && current.Name != "" {
		if current.Port == 0 {
			current.Port = 22
		}
		hosts = append(hosts, current)
	}

	if err := scanner.Err(); err != nil {
		return hosts, err
	}

	return hosts, nil
}

// supplementaryHostsFile is the YAML structure for hosts.yml
type supplementaryHostsFile struct {
	Hosts []*SSHHost `yaml:"hosts"`
}

// loadSupplementaryHosts reads hosts from a YAML config file
func loadSupplementaryHosts(configFile string) ([]*SSHHost, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var hostsFile supplementaryHostsFile
	if err := yaml.Unmarshal(data, &hostsFile); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", configFile, err)
	}

	// Set default port for any hosts that don't specify one
	for _, h := range hostsFile.Hosts {
		if h.Port == 0 {
			h.Port = 22
		}
		h.KeyPath = expandTilde(h.KeyPath)
	}

	return hostsFile.Hosts, nil
}

// saveSupplementaryHosts writes hosts to the YAML config file
func saveSupplementaryHosts(configFile string, hosts []*SSHHost) error {
	hostsFile := supplementaryHostsFile{Hosts: hosts}
	data, err := yaml.Marshal(&hostsFile)
	if err != nil {
		return err
	}

	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(configFile, data, 0644)
}

// mergeHosts merges two host lists, with supplementary hosts taking precedence
// on name collisions (supplementary hosts override SSH config entries)
func mergeHosts(sshHosts, supplementaryHosts []*SSHHost) []*SSHHost {
	seen := make(map[string]*SSHHost)

	// Add SSH config hosts first
	for _, h := range sshHosts {
		seen[h.Name] = h
	}

	// Supplementary hosts override
	for _, h := range supplementaryHosts {
		seen[h.Name] = h
	}

	// Build ordered result: SSH config order first, then supplementary additions
	result := make([]*SSHHost, 0, len(seen))
	added := make(map[string]bool)

	for _, h := range sshHosts {
		if !added[h.Name] {
			result = append(result, seen[h.Name])
			added[h.Name] = true
		}
	}

	for _, h := range supplementaryHosts {
		if !added[h.Name] {
			result = append(result, h)
			added[h.Name] = true
		}
	}

	return result
}

// expandTilde replaces a leading ~/ with the user's home directory
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home := os.Getenv("HOME")
		if home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// CheckReachability does a TCP dial to host:port with a 2-second timeout.
// Returns true if the port is open (host is reachable).
func CheckReachability(host *SSHHost) bool {
	addr := fmt.Sprintf("%s:%d", host.Hostname, host.Port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
