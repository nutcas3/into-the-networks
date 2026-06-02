package auth

import (
	"github.com/sirupsen/logrus"
)

type LogrusLogger struct {
	entry *logrus.Entry
}

func NewLogrusLogger() *LogrusLogger {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	return &LogrusLogger{
		entry: logrus.NewEntry(logger),
	}
}

func (l *LogrusLogger) SetLevel(level string) {
	switch level {
	case "debug":
		l.entry.Logger.SetLevel(logrus.DebugLevel)
	case "info":
		l.entry.Logger.SetLevel(logrus.InfoLevel)
	case "warn":
		l.entry.Logger.SetLevel(logrus.WarnLevel)
	case "error":
		l.entry.Logger.SetLevel(logrus.ErrorLevel)
	default:
		l.entry.Logger.SetLevel(logrus.InfoLevel)
	}
}

func (l *LogrusLogger) Info(args ...any) {
	l.entry.Info(args...)
}

func (l *LogrusLogger) Infof(format string, args ...any) {
	l.entry.Infof(format, args...)
}

func (l *LogrusLogger) Warn(args ...any) {
	l.entry.Warn(args...)
}

func (l *LogrusLogger) Warnf(format string, args ...any) {
	l.entry.Warnf(format, args...)
}

func (l *LogrusLogger) Error(args ...any) {
	l.entry.Error(args...)
}

func (l *LogrusLogger) Errorf(format string, args ...any) {
	l.entry.Errorf(format, args...)
}

func (l *LogrusLogger) Debug(args ...any) {
	l.entry.Debug(args...)
}

func (l *LogrusLogger) Debugf(format string, args ...any) {
	l.entry.Debugf(format, args...)
}

func (l *LogrusLogger) WithField(key string, value any) Logger {
	return &LogrusLogger{
		entry: l.entry.WithField(key, value),
	}
}

func (l *LogrusLogger) WithFields(fields map[string]any) Logger {
	return &LogrusLogger{
		entry: l.entry.WithFields(fields),
	}
}
