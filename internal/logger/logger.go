package logger

import (
	"fmt"
	"log"
	"os"
)

type Logger struct {
	log *log.Logger
}

var globalLogger *Logger

func newLogger() *Logger {
	globalLogger = &Logger{
		log: log.New(os.Stdout, "", log.Ltime|log.Lshortfile),
	}
	return globalLogger
}

func (l *Logger) info(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	l.log.Printf("\033[32mINFO\033[0m: %s", message)
}

func (l *Logger) error(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	l.log.Printf("\033[31mERROR\033[0m: %s", message)
}

func (l *Logger) warn(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	l.log.Printf("\033[33mWARN\033[0m: %s", message)
}

func (l *Logger) debug(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	l.log.Printf("\033[34mDEBUG\033[0m: %s", message)
}

func (l *Logger) success(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	l.log.Printf("\033[36mSUCCESS\033[0m: %s", message)
}

func get() *Logger {
	if globalLogger == nil {
		return newLogger()
	}
	return globalLogger
}

func Info(format string, v ...interface{})    { get().info(format, v...) }
func Error(format string, v ...interface{})   { get().error(format, v...) }
func Warn(format string, v ...interface{})    { get().warn(format, v...) }
func Debug(format string, v ...interface{})   { get().debug(format, v...) }
func Success(format string, v ...interface{}) { get().success(format, v...) }
