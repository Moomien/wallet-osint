package interceptor

import (
	log "arkham_checker/checker/logger"
	"arkham_checker/checker/session"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

const (
	selectorChatInput  = "div[contenteditable='true']"
	selectorLimitMsg   = "#last-reply-container >> text=Достигнут лимит сообщений"
	selectorStopButton = "button[aria-label*='Stop']"
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
	//получаем валидную сессию
	cookie, err := i.session.GetSession()
	if err != nil {
		return nil, fmt.Errorf("не удалось получить сессию: %w", err)
	}

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
		if strings.Contains(request.URL(), "responses") {
			slog.Info("Найден нужный запрос", "method", request.Method(), "url", request.URL())
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
		return nil, fmt.Errorf("не удалось перейти на grok.com: %w", err)
	}

	msgs := []string{"Hey grok", "LOL"}
	for _, msg := range msgs {
		if err := i.sendMessage(ctx, page, msg); err != nil {
			return nil, fmt.Errorf("не удалось отправить сообщение %q: %w", msg, err)
		}
	}

	select {
	case req := <-captured:
		return req, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(45 * time.Second):
		return nil, fmt.Errorf("таймаут ожидания запроса responses")
	}
}

func (i *Interceptor) sendMessage(ctx context.Context, page playwright.Page, msg string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	i.log.Info("Нажимаю на поле ввода")
	textarea := page.Locator(selectorChatInput).First()
	if err := textarea.Click(); err != nil {
		return fmt.Errorf("не удалось кликнуть по полю ввода: %w", err)
	}

	i.log.Info("Заполняю поле ввода", "msg", msg)
	if err := textarea.Fill(msg); err != nil {
		return fmt.Errorf("не удалось заполнить сообщение: %w", err)
	}

	i.log.Info("Нажимаю ENTER")
	if err := page.Keyboard().Press("Enter"); err != nil {
		return fmt.Errorf("не удалось нажать Enter: %w", err)
	}

	// Ожидаем начала и завершения генерации ответа через Locator
	stopBtn := page.Locator(selectorStopButton)

	i.log.Info("Ожидаю начала ответа...")
	_ = stopBtn.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})

	i.log.Info("Ожидаю завершения генерации ответа...")
	_ = stopBtn.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateHidden,
		Timeout: playwright.Float(60000),
	})

	i.log.Info("Проверяю наличие лимита")
	// Проверяем наличие лимита без долгого ожидания.
	limitLocator := page.Locator(selectorLimitMsg)
	if visible, _ := limitLocator.IsVisible(); visible {
		return fmt.Errorf("достигнут лимит сообщений аккаунта")
	}

	return nil
}
