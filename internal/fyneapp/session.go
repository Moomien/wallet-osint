package fyneapp

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Session представляет сохраненную сессию работы
type Session struct {
	Name string `json:"name"`
	Rows []Row  `json:"rows"`
}

// getSessionsDir возвращает путь к директории с сессиями
// Пытается найти корень проекта (где есть go.mod) или использует текущую директорию
func getSessionsDir() (string, error) {
	// Пробуем найти корень проекта по go.mod
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Идем вверх по директориям пока не найдем go.mod
	dir := currentDir
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			// Нашли go.mod - это корень проекта
			return filepath.Join(dir, "data", "app-sessions"), nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Дошли до корня файловой системы
			break
		}
		dir = parent
	}

	// Если не нашли go.mod, используем текущую директорию
	return filepath.Join(currentDir, "data", "app-sessions"), nil
}

// SaveSession сохраняет сессию в файл
func SaveSession(filename string, session *Session) error {
	sessionsDir, err := getSessionsDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return err
	}

	// Если передан полный путь, используем его
	// Если только имя файла, добавляем путь к директории сессий
	path := filename
	if filepath.Dir(filename) == "." {
		path = filepath.Join(sessionsDir, filename)
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadSession загружает сессию из файла
func LoadSession(path string) (*Session, error) {
	// Если путь не абсолютный, ищем в директории сессий
	if !filepath.IsAbs(path) && filepath.Dir(path) == "." {
		sessionsDir, err := getSessionsDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(sessionsDir, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// GetSessions возвращает список доступных сессий
func GetSessions() ([]string, error) {
	sessionsDir, err := getSessionsDir()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		// Создаем директорию если её нет
		if err := os.MkdirAll(sessionsDir, 0755); err != nil {
			return nil, err
		}
		return []string{}, nil
	}

	files, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, err
	}

	var sessions []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			sessions = append(sessions, file.Name())
		}
	}

	return sessions, nil
}
