package logger

import (
	"fmt"
	"log"
	"os"
)

type Logger struct {
	info  *log.Logger
	error *log.Logger
}

func New() *Logger {
	flags := log.Ldate | log.Ltime
	return &Logger{
		info:  log.New(os.Stdout, "[INFO]  ", flags),
		error: log.New(os.Stderr, "[ERROR] ", flags),
	}
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.info.Println(fmt.Sprintf(format, args...))
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.error.Println(fmt.Sprintf(format, args...))
}

func (l *Logger) Fatal(format string, args ...interface{}) {
	l.error.Fatal(fmt.Sprintf(format, args...))
}
