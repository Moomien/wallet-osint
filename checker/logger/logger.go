package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

func NewLogger(filename string) (*Logger, error) {
	logger, logfile, err := newLogger(filename)
	if err != nil {
		return nil, fmt.Errorf("HE получилось создать логгер %s. Error: %w", filename, err)
	}
	return &Logger{
		Log:     logger,
		logfile: logfile,
	}, nil
}

// создает новый логгер
func newLogger(name string) (*slog.Logger, *os.File, error) {

	dir, err := os.Getwd()
	if err != nil {
		return nil, nil, fmt.Errorf("HE удалось получить путь рабочей директории")
	}

	if strings.Contains(dir, name) {
		dir = filepath.Dir(dir)
	}

	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("HE удалось создать директорию логов: %w", err)
	}

	logpath := filepath.Join(logDir, strings.TrimSpace(name)+".log")
	file, err := os.OpenFile(logpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("HE удалось открыть файл лога: %w", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, file)
	//чтобы добавить debug логи нужно прописать
	logger := slog.New(slog.NewJSONHandler(multiWriter, nil)).With("component", name)

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
