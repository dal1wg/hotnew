package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"hotnew/internal/config"
)

type Logger struct {
	logger  *log.Logger
	output  string
	file    *os.File
	level   string
	mu      sync.Mutex
}

var (
	globalLogger *Logger
	once         sync.Once
)

func Init(cfg config.LoggingConfig) error {
	var err error
	once.Do(func() {
		globalLogger, err = newLogger(cfg)
	})
	return err
}

func newLogger(cfg config.LoggingConfig) (*Logger, error) {
	l := &Logger{
		output: cfg.Output,
		level:  cfg.Level,
	}

	if cfg.Output == "file" {
		dir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create log directory: %w", err)
		}

		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		l.file = file
		l.logger = log.New(file, "", log.LstdFlags|log.Lshortfile)
	} else {
		l.logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
	}

	return l, nil
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func Info(format string, v ...any) {
	globalLogger.log("INFO", format, v...)
}

func Debug(format string, v ...any) {
	globalLogger.log("DEBUG", format, v...)
}

func Warn(format string, v ...any) {
	globalLogger.log("WARN", format, v...)
}

func Error(format string, v ...any) {
	globalLogger.log("ERROR", format, v...)
}

func Fatal(format string, v ...any) {
	globalLogger.log("FATAL", format, v...)
	os.Exit(1)
}

func Close() error {
	if globalLogger != nil {
		return globalLogger.Close()
	}
	return nil
}

func getLevelPriority(level string) int {
	switch level {
	case "debug":
		return 0
	case "verbose":
		return 1
	case "info":
		return 2
	case "notice":
		return 3
	case "warning":
		return 4
	case "error":
		return 5
	case "fatal":
		return 6
	default:
		return 2 // default to info
	}
}

func (l *Logger) shouldLog(level string) bool {
	// 转换为小写进行比较
	lowerLevel := level
	if len(lowerLevel) > 0 {
		lowerLevel = strings.ToLower(lowerLevel)
	}
	return getLevelPriority(lowerLevel) >= getLevelPriority(l.level)
}

func (l *Logger) log(level, format string, v ...any) {
	if !l.shouldLog(level) {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	msg := fmt.Sprintf(format, v...)
	l.logger.Printf("[%s] %s - %s", level, time.Now().Format(time.RFC3339), msg)
}
