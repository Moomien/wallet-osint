package grokclient

import (
	paste "arkham_checker/checker/grokclient/paste_service"
	"arkham_checker/checker/interceptor"
	log "arkham_checker/checker/logger"
	"encoding/json"
	"strings"

	"context"
	_ "embed"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
)

// промпт для отправки нейронке
//
//go:embed prompt.txt
var prompt string

type GrokError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

func (e *GrokError) Error() string {
	msg := fmt.Sprintf("grok: API error. StatusCode: %v, Message: %v", e.StatusCode, e.Message)
	if e.RetryAfter > 0 {
		return fmt.Sprintf("grok: API error. StatusCode: %v, Message: %v, Retry After: %v", e.StatusCode, e.Message, e.RetryAfter)
	}
	return msg
}

type GrokClient struct {
	Paste  paste.Paste
	client *resty.Client
	prompt string
	log    *log.Logger
}

func NewGrokClient() (*GrokClient, error) {
	p, err := paste.NewPastebin()
	if err != nil {
		return nil, fmt.Errorf("создание инстанса пастебина: %w", err)
	}
	c := resty.New()

	logger, err := log.NewLogger("grokclient")
	if err != nil {
		return nil, fmt.Errorf("создание логгера grokclient: %w", err)
	}

	gc := &GrokClient{
		Paste:  p,
		client: c,
		prompt: prompt,
		log:    logger,
	}

	return gc, nil
}

// отправляет текст в грок и сохраняет в сервис паст и выдаёт URL на пасту
func (x *GrokClient) SendMessage(ctx context.Context, request *interceptor.CapturedRequest, userurl string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	//отправка запроса и получение ответа
	resp, err := x.grokRequest(request, userurl).Post(request.URL)
	if err != nil {
		return "", err
	}
	fmt.Println(resp.String())
	//обработка самых частых ошибок
	if resp.StatusCode() == 429 || resp.StatusCode() == 401 {
		x.log.Info("апи грока. статус код: %v", resp.StatusCode())
		gErr := &GrokError{
			StatusCode: resp.StatusCode(),
			Message:    resp.String(),
		}

		if resp.StatusCode() == 429 {
			if retry := resp.Header().Get("Retry-After"); retry != "" {
				seconds, err := strconv.Atoi(retry)
				if err != nil {
					return "", gErr
				}
				gErr.RetryAfter = time.Duration(seconds) * time.Second
			}
		}

		return "", gErr
	}
	//успех
	var pasteURL string
	if resp.StatusCode() == 200 {
		x.log.Info("Клиент успешно отправил запрос")
		res := gjson.Get(resp.String(), "result.modelResponse.message")
		if !res.Exists() {
			gErr := &GrokError{
				StatusCode: 200,
				Message:    "Код 200 но нейронка не ответила(?)",
				RetryAfter: 0,
			}
			return "", gErr
		}
		pasteURL, err = x.Paste.CreatePaste(res.String())
		if err != nil {
			return "", fmt.Errorf("сохранение текста нейронки в сервис паст: %w", err)
		}
	}

	return pasteURL, nil
}

// формирование запроса для последующей отправки
func (x *GrokClient) grokRequest(request *interceptor.CapturedRequest, userurl string) *resty.Request {
	msg := request.Body
	re := regexp.MustCompile(`("message"\s*:\s*")[^"]*("`)
	res := re.ReplaceAllString(msg, `${1}`+x.setPrompt(userurl)+`${2}`)

	x.log.Info("Заголовки из запроса", "headers", request.Headers)
	delete(request.Headers, "Content-Length")

	return x.client.NewRequest().SetHeaders(request.Headers).SetBody(res)
}

// берет юзернейм и подставляет его в промпт
func (x *GrokClient) setPrompt(userurl string) string {
	clean := strings.ReplaceAll(x.prompt, "\r", "")
	msg := fmt.Sprintf(clean, userurl, userurl)
	jsonBytes, _ := json.Marshal(msg)
	return string(jsonBytes[1 : len(jsonBytes)-1])
}
