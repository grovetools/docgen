package cmd

import (
	"github.com/grovetools/core/logging"
	"github.com/sirupsen/logrus"
)

var (
	log       = logging.NewLogger("grove-docgen")
	prettyLog = logging.NewPrettyLogger()
	ulog      = logging.NewUnifiedLogger("grove-docgen")
)

// getLogger returns the logrus.Logger for use with packages that expect it
func getLogger() *logrus.Logger {
	return log.Logger
}