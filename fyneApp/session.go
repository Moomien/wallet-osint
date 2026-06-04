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

// SaveSession сохраняет сессию в файл
func SaveSession(path string, session *Session) error {
	// TODO: Реализовать сохранение сессии
	// 1. Создать директорию, если не существует
	// 2. Сериализовать session в JSON
	// 3. Записать в файл

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadSession загружает сессию из файла
func LoadSession(path string) (*Session, error) {
	// TODO: Реализовать загрузку сессии
	// 1. Прочитать файл
	// 2. Десериализовать JSON в Session
	// 3. Вернуть результат

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
	// TODO: Реализовать получение списка сессий
	// 1. Определить директорию с сессиями
	// 2. Прочитать список файлов
	// 3. Отфильтровать .json файлы
	// 4. Вернуть список имен

	sessionsDir := "sessions" // или другая директория

	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
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
