package interceptor

import (
	log "arkham_checker/checker/logger"
	"arkham_checker/checker/session"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

const (
	selectorChatInput  = "div[contenteditable='true']"
	selectorStopButton = "button[aria-label*='Stop']"
	LimitMsg           = "limit"
)

type Interceptor struct {
	log     *log.Logger
	session *session.CacheSession
}

// CapturedRequest представляет данные, перехваченные из сетевого запроса.
type CapturedRequest struct {
	Headers map[string]string
	Body    string
	URL     string
}

func NewInterceptor(s *session.CacheSession) (*Interceptor, error) {
	logger, err := log.NewLogger("interceptor")
	if err != nil {
		return nil, fmt.Errorf("создание логгера: %w", err)
	}
	return &Interceptor{
		log:     logger,
		session: s,
	}, nil
}

func (i *Interceptor) Close() error {
	return i.log.Close()
}

// Capture запускает экземпляр Playwright, переходит в Grok, отправляет сообщения
// и перехватывает запрос responses на второе сообщение.
func (i *Interceptor) Capture(ctx context.Context, useragent string) (*CapturedRequest, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// получаем валидную сессию
		cookie, accountKey, err := i.session.GetSession()
		if err != nil {
			return nil, fmt.Errorf("не удалось получить сессию (возможно все в лимите): %w", err)
		}

		// Оборачиваем одну попытку в функцию, чтобы defer работал корректно на каждой итерации
		req, err := func() (*CapturedRequest, error) {
			browserCtx, err := i.session.Browser.NewContext(playwright.BrowserNewContextOptions{
				UserAgent: playwright.String(useragent),
			})
			if err != nil {
				return nil, fmt.Errorf("не удалось создать контекст браузера: %w", err)
			}
			defer browserCtx.Close()

			if err := browserCtx.AddCookies(cookie); err != nil {
				return nil, fmt.Errorf("не удалось добавить куки: %w", err)
			}
			page, err := browserCtx.NewPage()
			if err != nil {
				return nil, fmt.Errorf("не удалось создать новую страницу: %w", err)
			}
			defer page.Close()

			captured := make(chan *CapturedRequest, 1)
			// перехват запроса
			page.On("request", func(request playwright.Request) {
				if strings.HasSuffix(request.URL(), "/responses") {
					i.log.Info("Найден нужный запрос", "method", request.Method(), "url", request.URL())
					body, _ := request.PostData()
					captured <- &CapturedRequest{
						Headers: request.Headers(),
						Body:    body,
						URL:     request.URL(),
					}
				}
			})

			// переход в чат грока
			_, err = page.Goto("https://grok.com",
				playwright.PageGotoOptions{
					WaitUntil: playwright.WaitUntilStateNetworkidle,
				})
			if err != nil {
				i.log.Error("ошибка перехода на grok.com, пробуем следующий аккаунт", "err", err)
				return nil, nil
			}

			msgs := []string{"Hey grok", "LOL"}
			for _, msg := range msgs {
				if err := i.sendMessage(ctx, page, msg); err != nil {
					if strings.Contains(err.Error(), LimitMsg) {
						waitTime := parseWaitTime(err.Error())
						i.log.Warn("Обнаружен лимит сообщений, меняем аккаунт", "account", accountKey, "wait", waitTime.String())

						// Лимит пробуем следующий
						i.session.MarkInvalid(accountKey, waitTime)
						_ = page.Close()
						return nil, nil
					}
					return nil, fmt.Errorf("не удалось отправить сообщение %q: %w", msg, err)
				}
			}

			select {
			case req := <-captured:
				return req, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(45 * time.Second):
				i.log.Warn("таймаут ожидания запроса, пробуем другой аккаунт")
				return nil, nil
			}
		}()
		if err != nil {
			return nil, err
		}
		if req != nil {
			return req, nil
		}
		i.log.Info("Попытка завершена, перехожу к следующей итерации цикла...")
	}
}

// playwright отправляет сообщение
func (i *Interceptor) sendMessage(ctx context.Context, page playwright.Page, msg string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	i.log.Info("Ожидаю поле ввода")
	textarea := page.Locator(selectorChatInput).First()
	if err := textarea.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10000),
	}); err != nil {
		return fmt.Errorf("поле ввода не появилось: %w", err)
	}

	i.log.Info("Фокусируюсь и заполняю сообщение", "msg", msg)
	if err := textarea.Focus(); err != nil {
		return fmt.Errorf("не удалось сфокусироваться на поле ввода: %w", err)
	}

	if err := textarea.Type(msg); err != nil {
		return fmt.Errorf("не удалось ввести сообщение: %w", err)
	}

	i.log.Info("Нажимаю ENTER")
	if err := page.Keyboard().Press("Enter"); err != nil {
		return fmt.Errorf("не удалось нажать Enter: %w", err)
	}

	stopBtn := page.Locator(selectorStopButton)
	// Ищем плашку лимита по ключевым фразам
	limitMsg := page.Locator("text=/limit (reached|is gone)/i")

	i.log.Info("Ожидаю начала ответа или сообщения о лимите...")
	// Ждем либо кнопку остановки (значит ответ пошел), либо сообщение о лимите
	combined := stopBtn.Or(limitMsg)
	if err := combined.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(15000),
	}); err != nil {
		return fmt.Errorf("grok не начал отвечать после отправки сообщения: %w", err)
	}

	// Если видим сообщение о лимите - выходим с ошибкой
	if visible, _ := limitMsg.IsVisible(); visible {
		text, _ := limitMsg.InnerText()
		i.log.Info("Обнаружен лимит сообщений", "text", text)
		return fmt.Errorf("достигнут лимит сообщений: %s, %s", LimitMsg, text)
	}

	i.log.Info("Ожидаю завершения генерации ответа...")
	if err := stopBtn.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateHidden,
		Timeout: playwright.Float(90000),
	}); err != nil {
		return fmt.Errorf("grok не завершил генерацию ответа вовремя: %w", err)
	}

	i.log.Info("Ожидаю готовности чата к следующему сообщению")
	// Дожидаемся, что поле ввода снова готово
	if err := textarea.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	}); err != nil {
		i.log.Warn("Поле ввода не появилось быстро, продолжаем", "err", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(800 * time.Millisecond):
	}

	return nil
}

func parseWaitTime(errMsg string) time.Duration {
	errMsg = strings.ToLower(errMsg)
	re := regexp.MustCompile(`(\d+)\s+(hour|minute|second)`)
	matches := re.FindAllStringSubmatch(errMsg, -1)

	var totalDuration time.Duration
	for _, match := range matches {
		val, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		unit := match[2]

		switch {
		case strings.HasPrefix(unit, "hour"):
			totalDuration += time.Duration(val) * time.Hour
		case strings.HasPrefix(unit, "minute"):
			totalDuration += time.Duration(val) * time.Minute
		case strings.HasPrefix(unit, "second"):
			totalDuration += time.Duration(val) * time.Second
		}
	}

	if totalDuration == 0 {
		return 24 * time.Hour
	}
	return totalDuration
}
