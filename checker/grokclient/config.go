package grokclient

import (
	"arkham_checker/checker/interceptor"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type RequestProvider interface {
	get(ctx context.Context, useragent string) (*interceptor.CapturedRequest, error)
	setMode(mode string)
	length() int
	buildbody(request *interceptor.CapturedRequest, username string, prompt string) (string, error)
	parseResponse(body string) (string, error)
}

type GrokConfig struct {
	Mode        string                       `json:"mode"`
	Request     *interceptor.CapturedRequest `json:"request"`
	filepath    string                       `json:"-"`
	interceptor *interceptor.Interceptor     `json:"-"`
	mu          sync.RWMutex                 `json:"-"`
	isTaken     bool
}

// грузим стандартный конфиг(static)
func LoadConfig(path string, i *interceptor.Interceptor) (*GrokConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("загрузка json файла: %w", err)
	}

	var config GrokConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshall json: %w", err)
	}
	config.filepath = path
	config.interceptor = i
	return &config, nil
}

// получаем сессию, в зависимости от мода в конфиге
func (config *GrokConfig) get(ctx context.Context, useragent string) (*interceptor.CapturedRequest, error) {
	config.mu.RLock()
	mode := config.Mode
	config.mu.RUnlock()

	if mode == "static" {
		return config.Request, nil
	}

	if mode == "interceptor" {
		return config.interceptor.Capture(ctx, useragent)
	}
	return nil, errors.New("не получилось получить сессию")
}

// изменяем мод в случае ошибок одного из модов - протуханий сессий итд
func (config *GrokConfig) setMode(mode string) {
	config.mu.Lock()
	defer config.mu.Unlock()
	config.Mode = mode
}

// строим body для запроса в клиенте
func (config *GrokConfig) buildbody(request *interceptor.CapturedRequest, username string, prompt string) (string, error) {
	config.mu.RLock()
	mode := config.Mode
	config.mu.RUnlock()

	if mode == "static" {
		return sjson.Set(request.Body, "input.0.content.0.text", formatPrompt(username, prompt))
	}

	re := regexp.MustCompile(`("message"\s*:\s*")[^"]*(")`)
	result := re.ReplaceAllString(request.Body, `${1}`+formatPrompt(username, prompt)+`${2}`)
	return result, nil
}

// парсим ответ от клиента
func (config *GrokConfig) parseResponse(resp string) (string, error) {
	var text string
	config.mu.RLock()
	mode := config.Mode
	config.mu.RUnlock()
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

// количество сессий
func (config *GrokConfig) length() int {
	return config.interceptor.Length()
}

// берет юзернейм и подставляет его в промпт
func formatPrompt(username, prompt string) string {
	clean := strings.ReplaceAll(prompt, "\r", "")
	msg := fmt.Sprintf(clean, username, username)
	jsonBytes, _ := json.Marshal(msg)
	return string(jsonBytes[1 : len(jsonBytes)-1])
}
