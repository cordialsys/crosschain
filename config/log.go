package config

import (
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Call this to configure the logrus logger for a cordial systems program.
// Set the loglevel, formatter, color options, etc.

const CordialLogLevel = "CORDIAL_LOG_LEVEL"
const CordialLogFormat = "CORDIAL_LOG_FORMAT"

func ConfigureLogger(levelMaybe ...string) {
	time.Local = time.FixedZone("UTC", 0)
	logrus.SetLevel(logrus.InfoLevel)

	level := os.Getenv(CordialLogLevel)
	if len(levelMaybe) > 0 {
		level = levelMaybe[0]
	}

	switch level {
	case "trace":
		logrus.SetLevel(logrus.TraceLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	format := os.Getenv(CordialLogFormat)
	if format == "" {
		format = "color-text"
	}
	switch strings.ToLower(format) {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	case "text":
		logrus.SetFormatter(&logrus.TextFormatter{
			DisableColors: true,
		})
	case "color-text":
		logrus.SetFormatter(&logrus.TextFormatter{
			DisableColors: false,
			ForceColors:   true,
		})
	default:
		logrus.WithFields(logrus.Fields{
			"format":  format,
			"options": []string{"json", "text", "color-text"},
		}).Warn("unknown format")
	}

}
