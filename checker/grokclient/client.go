package grokclient

import (
	log "arkham_checker/checker/logger"
	"errors"

	"context"
	_ "embed"
	"fmt"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
)

type GrokError struct {
	StatusCode    int
	Message       string
	RetryAfter    time.Duration
	SessionExpire bool
}

func (e *GrokError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("grok: API ошибка. StatusCode: %v, Message: %v, Retry After: %v", e.StatusCode, e.Message, e.RetryAfter)
	}
	if e.SessionExpire {
		return fmt.Sprintf("Сессия протухла (статус %d)", e.StatusCode)
	}

	return fmt.Sprintf("grok: API ошибка. StatusCode: %v, Message: %v", e.StatusCode, e.Message)
}

type GrokClient struct {
	client    *resty.Client
	useragent string
	prompt    string
	log       *log.Logger
}

func NewGrokClient(prompt, useragent string) (*GrokClient, error) {
	c := resty.New()

	logger, err := log.NewLogger("grokclient")
	if err != nil {
		return nil, fmt.Errorf("создание логгера grokclient: %w", err)
	}

	gc := &GrokClient{
		client:    c,
		useragent: useragent,
		prompt:    prompt,
		log:       logger,
	}

	return gc, nil
}

func (x *GrokClient) Close() error {
	return x.log.Close()
}

// отправляет текст в грок и получает готовую строку - ответ
// provider - наш конфиг
// attempt должен быть 0
func (x *GrokClient) SendMessage(ctx context.Context, provider RequestProvider, username string, attempt int) (string, error) {
	if attempt > provider.length() {
		return "", errors.New("Попробовали все способы, все куки, ничего не сработало")
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	//будем пробовать по дефолту static, в случае протухания сессии переключимся на interceptor
	request, err := provider.get(ctx, x.useragent)
	if err != nil {
		return "", err
	}
	body, err := provider.buildbody(request, username, x.prompt)
	if err != nil {
		return "", err
	}

	//отправка запроса и получение ответа
	resp, err := x.client.NewRequest().SetHeaders(request.Headers).SetBody(body).Post(request.URL)
	if err != nil {
		return "", err
	}
	fmt.Println(resp.String())

	//обработка самых частых ошибок
	if resp.StatusCode() == 429 || resp.StatusCode() == 401 || resp.StatusCode() == 403 {
		x.log.Info("апи грока. статус код: %d", resp.StatusCode())
		//если 401 то точно протухла
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
				x.log.Warn("ушли в ретрай", "retry-after", time.Duration(seconds)*time.Second)
				gErr.RetryAfter = time.Duration(seconds) * time.Second
				gErr.SessionExpire = false
			}
		}

		if resp.StatusCode() == 401 || resp.StatusCode() == 403 {
			gErr.SessionExpire = true
			x.log.Warn("Сессия протухла, пробую перехватчик.....")
			//выставляем другой мод
			provider.setMode("interceptor")
			return x.SendMessage(ctx, provider, username, attempt+1)
		}

		return "", gErr
	}

	//Успех
	var text string
	if resp.StatusCode() == 200 {
		x.log.Info("Клиент успешно отправил запрос, ответ : 200")
		//обрабатываем ответ
		text, err = provider.parseResponse(resp.String())
		if err != nil {
			return "", err
		}
	}

	return text, nil
}
