package logger

import (
	"io"
	"log"
	"os"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

type LogLevel int

const (
	ErrorLevel LogLevel = iota
	WarnLevel
	InfoLevel
	DebugLevel
)

var currentLevel LogLevel = InfoLevel

var (
	errorLogger *log.Logger
	warnLogger  *log.Logger
	infoLogger  *log.Logger
	debugLogger *log.Logger
)

type Options struct {
	Level       string
	Output      string
	LogFilePath string
	MaxSizeMB   int
	MaxBackups  int
	MaxAgeDays  int
}

func Init(opts Options) {
	SetLevel(opts.Level)

	var output io.Writer = os.Stdout

	if opts.Output == "file" && opts.LogFilePath != "" {
		output = &lumberjack.Logger{
			Filename:   opts.LogFilePath,
			MaxSize:    opts.MaxSizeMB, // megabytes
			MaxBackups: opts.MaxBackups,
			MaxAge:     opts.MaxAgeDays, // days
			Compress:   true,
		}
	}

	errorLogger = log.New(output, "[ERROR] ", log.LstdFlags)
	warnLogger = log.New(output, "[WARN]  ", log.LstdFlags)
	infoLogger = log.New(output, "[INFO]  ", log.LstdFlags)
	debugLogger = log.New(output, "[DEBUG] ", log.LstdFlags)
}

func SetLevel(level string) {
	switch strings.ToLower(level) {
	case "error":
		currentLevel = ErrorLevel
	case "warn":
		currentLevel = WarnLevel
	case "info":
		currentLevel = InfoLevel
	case "debug":
		currentLevel = DebugLevel
	default:
		currentLevel = InfoLevel
	}
}

func Error(format string, v ...any) {
	if currentLevel >= ErrorLevel && errorLogger != nil {
		errorLogger.Printf(format, v...)
	}
}

func Warn(format string, v ...any) {
	if currentLevel >= WarnLevel && warnLogger != nil {
		warnLogger.Printf(format, v...)
	}
}

func Info(format string, v ...any) {
	if currentLevel >= InfoLevel && infoLogger != nil {
		infoLogger.Printf(format, v...)
	}
}

func Debug(format string, v ...any) {
	if currentLevel >= DebugLevel && debugLogger != nil {
		debugLogger.Printf(format, v...)
	}
}
