package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Log — интерфейс логгера для инъекции зависимостей
type Log interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
	Close() error
}

type Logger struct {
	Log     *slog.Logger
	logfile *os.File
}

func NewLogger(componentName string) (*Logger, error) {
	logger, logfile, err := newLogger(componentName)
	if err != nil {
		return nil, fmt.Errorf("не получилось создать логгер %s. Error: %w", componentName, err)
	}
	return &Logger{
		Log:     logger,
		logfile: logfile,
	}, nil
}

// создает новый логгер с отдельным файлом для компонента
func newLogger(componentName string) (*slog.Logger, *os.File, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось получить путь рабочей директории")
	}

	logDir := filepath.Join(dir, "data", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("не удалось создать директорию логов: %w", err)
	}

	logpath := filepath.Join(logDir, componentName+".log")
	file, err := os.OpenFile(logpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось открыть файл лога: %w", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, file)
	logger := slog.New(slog.NewJSONHandler(multiWriter, nil)).With("component", componentName)

	return logger, file, nil
}

// методы-прокси чтобы можно было вызывать logger.Info() и.тд
func (l *Logger) Info(msg string, args ...any) {
	l.Log.Info(msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.Log.Warn(msg, args...)
}
func (l *Logger) Error(msg string, args ...any) {
	l.Log.Error(msg, args...)
}

func (l *Logger) Debug(msg string, args ...any) {
	l.Log.Debug(msg, args...)
}

// закрывает файл лога
func (log *Logger) Close() error {
	if log.logfile != nil {
		return log.logfile.Close()
	}
	return nil
}
