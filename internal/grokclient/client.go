package grokclient

import (
	log "arkham_checker/internal/logger"
	"errors"
	"sync"

	"golang.org/x/time/rate"

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
	client         *resty.Client
	useragent      string
	prompt         string
	currentSession string
	mu             sync.Mutex
	rateLimiter    *restyLimiter
	log            log.Log
}

// restyLimiter — адаптер rate.Limiter для resty (Wait вместо Allow)
type restyLimiter struct {
	limiter *rate.Limiter
}

func newRestyLimiter(rps, burst int) *restyLimiter {
	return &restyLimiter{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
}

func (r *restyLimiter) Allow() bool {
	if err := r.limiter.Wait(context.Background()); err != nil {
		return false
	}
	return true
}

func NewGrokClient(prompt, useragent string) (*GrokClient, error) {
	logger, err := log.NewLogger("grokclient")
	if err != nil {
		return nil, fmt.Errorf("создание логгера grokclient: %w", err)
	}

	limiter := newRestyLimiter(1, 1)
	c := resty.New()
	c.SetRateLimiter(limiter)

	gc := &GrokClient{
		client:         c,
		useragent:      useragent,
		prompt:         prompt,
		currentSession: "default",
		rateLimiter:    limiter,
		log:            logger,
	}

	return gc, nil
}

func (x *GrokClient) Close() error {
	return x.log.Close()
}

// устанавливает активную сессию (потокобезопасно)
func (x *GrokClient) SetSession(sessionName string) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.currentSession = sessionName
}

// возвращает имя текущей сессии (потокобезопасно)
func (x *GrokClient) GetSession() string {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.currentSession
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

	x.mu.Lock()
	sessionName := x.currentSession
	x.mu.Unlock()

	x.log.Info("Используем сессию", "session", sessionName)

	//будем пробовать по дефолту static, в случае протухания сессии переключимся на interceptor
	request, err := provider.get(ctx, x.useragent, sessionName)
	if err != nil {
		return "", err
	}
	body, err := provider.buildbody(request, username, x.prompt, sessionName)
	if err != nil {
		return "", err
	}

	//отправка запроса и получение ответа
	resp, err := x.client.NewRequest().SetHeaders(request.Headers).SetBody(body).Post(request.URL)
	if err != nil {
		return "", err
	}

	//обработка самых частых ошибок
	if resp.StatusCode() == 429 || resp.StatusCode() == 401 || resp.StatusCode() == 403 {
		x.log.Info("API Grok вернул ошибку", "status_code", resp.StatusCode())
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
			x.log.Warn("Сессия протухла, пробую переключиться на другую", "session", sessionName)

			// Пытаемся переключиться на другую доступную сессию
			sessions := provider.GetSessionNames()
			for _, name := range sessions {
				if name != sessionName {
					x.log.Info("Переключаюсь на сессию", "new_session", name)
					x.SetSession(name)
					return x.SendMessage(ctx, provider, username, attempt+1)
				}
			}

			// Если других сессий нет, пробуем переключить текущую на interceptor
			if err := provider.setMode(sessionName, "interceptor"); err == nil {
				x.log.Info("Переключил сессию на режим interceptor", "session", sessionName)
				return x.SendMessage(ctx, provider, username, attempt+1)
			}

			return "", gErr
		}

		return "", gErr
	}

	//Успех
	var text string
	if resp.StatusCode() == 200 {
		x.log.Info("Клиент успешно отправил запрос, ответ : 200")
		//обрабатываем ответ
		text, err = provider.parseResponse(resp.String(), sessionName)
		if err != nil {
			return "", err
		}
	}

	return text, nil
}
