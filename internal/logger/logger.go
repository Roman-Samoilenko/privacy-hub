package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var levelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

var levelColors = map[LogLevel]string{
	DEBUG: "\033[34m", // Blue
	INFO:  "\033[32m", // Green
	WARN:  "\033[33m", // Yellow
	ERROR: "\033[31m", // Red
}

const (
	colorReset   = "\033[0m"
	colorSuccess = "\033[36m" // Cyan
)

type Logger struct {
	log   *log.Logger
	level LogLevel
	mu    sync.RWMutex
}

var globalLogger *Logger
var once sync.Once

func init() {
	once.Do(func() {
		globalLogger = &Logger{
			log:   log.New(os.Stdout, "", log.Ltime|log.Lshortfile),
			level: INFO,
		}
	})
}

func SetLevel(levelStr string) {
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()

	switch strings.ToLower(levelStr) {
	case "debug":
		globalLogger.level = DEBUG
	case "info":
		globalLogger.level = INFO
	case "warn", "warning":
		globalLogger.level = WARN
	case "error":
		globalLogger.level = ERROR
	default:
		globalLogger.level = INFO
	}
}

func (l *Logger) logf(level LogLevel, format string, v ...interface{}) {
	l.mu.RLock()
	currentLevel := l.level
	l.mu.RUnlock()

	if level < currentLevel {
		return
	}

	message := fmt.Sprintf(format, v...)
	color := levelColors[level]
	levelName := levelNames[level]

	l.log.Printf("%s%s%s: %s", color, levelName, colorReset, message)
}

func (l *Logger) successf(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	l.log.Printf("%sSUCCESS%s: %s", colorSuccess, colorReset, message)
}

func Debugf(format string, v ...interface{}) {
	globalLogger.logf(DEBUG, format, v...)
}

func Infof(format string, v ...interface{}) {
	globalLogger.logf(INFO, format, v...)
}

func Warnf(format string, v ...interface{}) {
	globalLogger.logf(WARN, format, v...)
}

func Errorf(format string, v ...interface{}) {
	globalLogger.logf(ERROR, format, v...)
}

func Successf(format string, v ...interface{}) {
	globalLogger.successf(format, v...)
}
