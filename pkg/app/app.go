package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sirupsen/logrus"

	"lazyscpsync/pkg/commands"
	"lazyscpsync/pkg/config"
	"lazyscpsync/pkg/gui"
	"lazyscpsync/pkg/i18n"
	"lazyscpsync/pkg/log"
)

// App is the main application struct
type App struct {
	Config      *config.AppConfig
	UserConfig  *config.UserConfig
	Log         *logrus.Entry
	OSCommand   *commands.OSCommand
	HostCommand *commands.SSHHostCommand
	SCPCommand  *commands.SCPCommand
	SyncManager *commands.SyncManager
	TeaProgram  *tea.Program
	TeaModel    gui.Model
	Tr          *i18n.TranslationSet
	ErrorChan   chan error
}

// NewApp creates and initializes the application
func NewApp() (*App, error) {
	// Initialize config
	appConfig, err := config.NewAppConfig()
	if err != nil {
		return nil, err
	}

	// Initialize logger
	logger := log.NewLogger(appConfig)

	// Load user config
	userConfig, err := config.LoadUserConfig(appConfig.HostsFile)
	if err != nil {
		logger.WithError(err).Warn("failed to load user config, using defaults")
		userConfig = &config.UserConfig{
			DefaultLocalPath:  "/home",
			DefaultRemotePath: "/tmp",
			SyncDebounceMs:    500,
		}
	}

	// Initialize translations
	tr := i18n.NewEnglishTranslations()

	// Initialize commands
	osCommand := commands.NewOSCommand(logger)
	hostCommand := commands.NewSSHHostCommand(appConfig, logger)
	scpCommand := commands.NewSCPCommand(osCommand, logger)
	syncManager := commands.NewSyncManager(osCommand, logger)

	// Initialize Bubble Tea model
	model := gui.NewModel(
		appConfig,
		userConfig,
		hostCommand,
		osCommand,
		scpCommand,
		syncManager,
		tr,
		logger,
	)

	// Create Bubble Tea program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	app := &App{
		Config:      appConfig,
		UserConfig:  userConfig,
		Log:         logger,
		OSCommand:   osCommand,
		HostCommand: hostCommand,
		SCPCommand:  scpCommand,
		SyncManager: syncManager,
		TeaProgram:  p,
		TeaModel:    model,
		Tr:          tr,
		ErrorChan:   make(chan error, 10),
	}

	return app, nil
}

// Run starts the application's main event loop
func (a *App) Run() error {
	if a.TeaProgram == nil {
		return nil
	}
	_, err := a.TeaProgram.Run()
	return err
}

// Close cleans up the application resources
func (a *App) Close() error {
	// Stop all sync sessions
	if a.SyncManager != nil {
		_ = a.SyncManager.Close()
	}
	// Bubble Tea cleanup happens automatically when program.Run() returns
	a.Log.Info("Application closed cleanly")
	return nil
}

// RunWithContext runs the app with a context for cancellation
func (a *App) RunWithContext(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		_ = a.Close()
	}()
	return a.Run()
}
