package grokclient

import (
	"arkham_checker/checker/interceptor"
	log "arkham_checker/checker/logger"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type RequestProvider interface {
	get(ctx context.Context, useragent string, sessionName string) (*interceptor.CapturedRequest, error)
	setMode(sessionName string, mode string) error
	length() int
	buildbody(request *interceptor.CapturedRequest, username string, prompt string, sessionName string) (string, error)
	parseResponse(body string, sessionName string) (string, error)
	GetSessionNames() []string
}

type SessionConfig struct {
	Mode    string                       `json:"mode"`
	Request *interceptor.CapturedRequest `json:"request"`
}

type GrokConfig struct {
	Sessions    map[string]*SessionConfig `json:"sessions"`
	filepath    string                    `json:"-"`
	interceptor *interceptor.Interceptor  `json:"-"`
	log         log.Log                   `json:"-"`
	mu          sync.RWMutex              `json:"-"`
}

// создаёт полный GrokConfig из папки с .txt файлами
func NewGrokConfig(dirPath string, i *interceptor.Interceptor) (*GrokConfig, error) {
	logger, err := log.NewLogger("grokclient")
	if err != nil {
		return nil, fmt.Errorf("создание логгера grokclient: %w", err)
	}

	sessions, err := LoadSessionsFromDirectory(dirPath, logger)
	if err != nil {
		logger.Close()
		return nil, err
	}

	config := &GrokConfig{
		Sessions:    sessions,
		filepath:    dirPath,
		interceptor: i,
		log:         logger,
	}

	logger.Info(fmt.Sprintf("Загружено сессий: %d", len(sessions)))
	return config, nil
}

// получаем сессию по имени, в зависимости от мода
func (config *GrokConfig) get(ctx context.Context, useragent string, sessionName string) (*interceptor.CapturedRequest, error) {
	config.mu.RLock()
	session, ok := config.Sessions[sessionName]
	config.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("сессия '%s' не найдена", sessionName)
	}

	if session.Mode == "static" {
		return session.Request, nil
	}

	if session.Mode == "interceptor" {
		return config.interceptor.Capture(ctx, useragent)
	}
	return nil, errors.New("не получилось получить сессию")
}

// изменяем мод для конкретной сессии
func (config *GrokConfig) setMode(sessionName string, mode string) error {
	config.mu.Lock()
	defer config.mu.Unlock()

	session, ok := config.Sessions[sessionName]
	if !ok {
		return fmt.Errorf("сессия '%s' не найдена", sessionName)
	}

	session.Mode = mode
	return nil
}

// строим body для запроса в клиенте
func (config *GrokConfig) buildbody(request *interceptor.CapturedRequest, username string, prompt string, sessionName string) (string, error) {
	config.mu.RLock()
	session, ok := config.Sessions[sessionName]
	mode := session.Mode
	config.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("сессия '%s' не найдена", sessionName)
	}

	if mode == "static" {
		body := request.Body
		// Добавляем prompt_cache_key для кэширования промпта
		body, _ = sjson.Set(body, "prompt_cache_key", sessionName)

		body, _ = sjson.Set(body, "input.0.content.0.text", formatPrompt(username, prompt))

		return body, nil
	}

	// Для interceptor mode - кэширование не поддерживается
	re := regexp.MustCompile(`("message"\s*:\s*")[^"]*(")`)
	result := re.ReplaceAllString(request.Body, `${1}`+formatPrompt(username, prompt)+`${2}`)
	return result, nil
}

// парсим ответ от клиента
func (config *GrokConfig) parseResponse(resp string, sessionName string) (string, error) {
	var text string
	config.mu.RLock()
	session, ok := config.Sessions[sessionName]
	mode := session.Mode
	config.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("сессия '%s' не найдена", sessionName)
	}

	if mode == "static" {
		scanner := bufio.NewScanner(strings.NewReader(resp))
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if gjson.Get(data, "type").String() == "response.output_text.done" {
				text = gjson.Get(data, "text").String()
				return text, nil
			}
		}
	}

	if mode == "interceptor" {
		text = gjson.Get(resp, "result.modelResponse.message").String()
		return text, nil
	}

	return "", errors.New("Body не пришел :(")
}

// количество попыток (для interceptor режима)
func (config *GrokConfig) length() int {
	if config.interceptor != nil {
		return config.interceptor.Length()
	}
	return len(config.Sessions)
}

// получить список всех сессий
func (config *GrokConfig) GetSessionNames() []string {
	config.mu.RLock()
	defer config.mu.RUnlock()

	names := make([]string, 0, len(config.Sessions))
	for name := range config.Sessions {
		names = append(names, name)
	}
	return names
}

func (config *GrokConfig) Close() error {
	if config.log != nil {
		return config.log.Close()
	}
	return nil
}

// берет юзернейм и подставляет его в промпт
func formatPrompt(username, prompt string) string {
	clean := strings.ReplaceAll(prompt, "\r", "")
	msg := strings.ReplaceAll(clean, "{{USERNAME}}", "@"+username)
	jsonBytes, _ := json.Marshal(msg)
	return string(jsonBytes[1 : len(jsonBytes)-1])
}
