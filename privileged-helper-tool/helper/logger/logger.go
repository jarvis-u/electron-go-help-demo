package logger

import (
	"fmt"
	"log"
	"os"
)

var logger *Logger

type Logger struct {
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
}

func init() {
	logger = &Logger{
		infoLogger: log.New(os.Stdout,
			"[INFO] ",
			log.Ldate|log.Ltime|log.Lshortfile),
		warnLogger: log.New(os.Stdout,
			"[WARN] ",
			log.Ldate|log.Ltime|log.Lshortfile),
		errorLogger: log.New(os.Stderr,
			"[ERROR] ",
			log.Ldate|log.Ltime|log.Lshortfile),
	}
}

func Info(format string, v ...interface{}) {
	logger.infoLogger.Output(2, fmt.Sprintf(format, v...))
}

func Warn(format string, v ...interface{}) {
	logger.warnLogger.Output(2, fmt.Sprintf(format, v...))
}

func Error(format string, v ...interface{}) {
	logger.errorLogger.Output(2, fmt.Sprintf(format, v...))
}
