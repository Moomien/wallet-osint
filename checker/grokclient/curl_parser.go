package grokclient

import (
	"arkham_checker/checker/interceptor"
	log "arkham_checker/checker/logger"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// парсит curl команду и возвращает SessionConfig
func ParseCurlCommand(curlCmd string) (*SessionConfig, error) {
	// Убираем переносы строк и лишние пробелы
	curlCmd = strings.ReplaceAll(curlCmd, "\\\n", " ")
	curlCmd = strings.ReplaceAll(curlCmd, "\\\r\n", " ")
	curlCmd = strings.TrimSpace(curlCmd)

	// Извлекаем URL
	urlRegex := regexp.MustCompile(`curl\s+'([^']+)'`)
	urlMatch := urlRegex.FindStringSubmatch(curlCmd)
	if len(urlMatch) < 2 {
		return nil, fmt.Errorf("не найден URL в curl команде")
	}
	url := urlMatch[1]

	// Извлекаем все заголовки
	headers := make(map[string]string)
	headerRegex := regexp.MustCompile(`-H\s+'([^:]+):\s*([^']*)'`)
	headerMatches := headerRegex.FindAllStringSubmatch(curlCmd, -1)
	for _, match := range headerMatches {
		if len(match) >= 3 {
			headerName := strings.TrimSpace(match[1])
			headerValue := strings.TrimSpace(match[2])
			headers[headerName] = headerValue
		}
	}

	// Извлекаем cookies из -b
	cookieRegex := regexp.MustCompile(`-b\s+'([^']+)'`)
	cookieMatch := cookieRegex.FindStringSubmatch(curlCmd)
	if len(cookieMatch) >= 2 {
		headers["Cookie"] = cookieMatch[1]
	}

	// Извлекаем body из --data-raw
	var body string
	bodyRegex := regexp.MustCompile(`--data-raw\s+'([^']+)'`)
	bodyMatch := bodyRegex.FindStringSubmatch(curlCmd)
	if len(bodyMatch) >= 2 {
		body = bodyMatch[1]
		// Проверяем что это валидный JSON
		var jsonCheck interface{}
		if err := json.Unmarshal([]byte(body), &jsonCheck); err != nil {
			return nil, fmt.Errorf("невалидный JSON в body: %w", err)
		}
	}

	// Создаем request
	request := &interceptor.CapturedRequest{
		Headers: headers,
		Body:    body,
		URL:     url,
	}

	return &SessionConfig{
		Mode:    "static",
		Request: request,
	}, nil
}

//	загружает все .txt файлы из папки и парсит их как curl команды
//
// Имя файла (без расширения) становится именем сессии
func LoadSessionsFromDirectory(dirPath string, logger *log.Logger) (map[string]*SessionConfig, error) {
	sessions := make(map[string]*SessionConfig)

	// Читаем все файлы в директории
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("чтение директории %s: %w", dirPath, err)
	}

	for _, file := range files {
		// Пропускаем директории и не .txt файлы
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".txt") {
			continue
		}

		filePath := filepath.Join(dirPath, file.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			if logger != nil {
				logger.Warn("[curl]не удалось прочитать %s: %v", file.Name(), err)
			}
			continue
		}

		// Парсим curl команду
		session, err := ParseCurlCommand(string(content))
		if err != nil {
			if logger != nil {
				logger.Warn("[curl]не удалось распарсить %s: %v", file.Name(), err)
			}
			continue
		}

		// Имя сессии = имя файла без расширения
		sessionName := strings.TrimSuffix(file.Name(), ".txt")
		sessions[sessionName] = session
		if logger != nil {
			logger.Info(fmt.Sprintf("[curl]Загружена сессия '%s' из %s", sessionName, file.Name()))
		}
	}

	if len(sessions) == 0 {
		return nil, fmt.Errorf("не найдено ни одной валидной сессии в %s", dirPath)
	}

	return sessions, nil
}
