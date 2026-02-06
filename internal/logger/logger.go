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

func (l *Logger) infof(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	l.log.Printf("\033[32mINFO\033[0m: %s", message)
}

func (l *Logger) errorf(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	l.log.Printf("\033[31mERROR\033[0m: %s", message)
}

func (l *Logger) warnf(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	l.log.Printf("\033[33mWARN\033[0m: %s", message)
}

func (l *Logger) debugf(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	l.log.Printf("\033[34mDEBUG\033[0m: %s", message)
}

func (l *Logger) successf(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	l.log.Printf("\033[36mSUCCESS\033[0m: %s", message)
}

func get() *Logger {
	if globalLogger == nil {
		return newLogger()
	}
	return globalLogger
}

func Infof(format string, v ...interface{})    { get().infof(format, v...) }
func Errorf(format string, v ...interface{})   { get().errorf(format, v...) }
func Warnf(format string, v ...interface{})    { get().warnf(format, v...) }
func Debugf(format string, v ...interface{})   { get().debugf(format, v...) }
func Successf(format string, v ...interface{}) { get().successf(format, v...) }
