package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"lazysync/pkg/config"

	"github.com/sirupsen/logrus"
)

// NewLogger returns a new logger
func NewLogger(cfg *config.AppConfig) *logrus.Entry {
	var log *logrus.Logger
	if os.Getenv("DEBUG") == "TRUE" {
		log = newDevelopmentLogger(cfg)
	} else {
		log = newProductionLogger()
	}

	log.Formatter = &logrus.JSONFormatter{}

	return log.WithFields(logrus.Fields{
		"version":   cfg.Version,
		"commit":    cfg.Commit,
		"buildDate": cfg.BuildDate,
	})
}

func getLogLevel() logrus.Level {
	strLevel := os.Getenv("LOG_LEVEL")
	level, err := logrus.ParseLevel(strLevel)
	if err != nil {
		return logrus.DebugLevel
	}
	return level
}

func newDevelopmentLogger(cfg *config.AppConfig) *logrus.Logger {
	log := logrus.New()
	log.SetLevel(getLogLevel())
	file, err := os.OpenFile(filepath.Join(cfg.ConfigDir, "development.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		fmt.Println("unable to log to file")
		os.Exit(1)
	}
	log.SetOutput(file)
	return log
}

func newProductionLogger() *logrus.Logger {
	log := logrus.New()
	log.Out = io.Discard
	log.SetLevel(logrus.ErrorLevel)
	return log
}
