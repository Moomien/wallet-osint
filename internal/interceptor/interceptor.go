package interceptor

import (
	log "arkham_checker/internal/logger"
	"arkham_checker/internal/session"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

const (
	LimitMsg      = "limit"
	HighDemandMsg = "high demand"
	maxRetries    = 5
)

type Interceptor struct {
	session *session.CacheSession
	log     log.Log
}

// представляет данные, перехваченные из сетевого запроса.
type CapturedRequest struct {
	Headers map[string]string `json:"Headers"`
	Body    string            `json:"Body"`
	URL     string            `json:"URL"`
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

//	запускает экземпляр Playwright, переходит в Grok, отправляет сообщения
//
// и перехватывает запрос responses на второе сообщение.
func (i *Interceptor) Capture(ctx context.Context, useragent string) (*CapturedRequest, error) {
	captured := make(chan *CapturedRequest, 10)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Очищаем канал перед новой попыткой
		select {
		case <-captured:
			i.log.Warn("Очищен старый запрос из канала перед новой попыткой")
		default:
		}

		i.log.Info("Попытка перехвата запроса", "attempt", attempt, "max", maxRetries)

		// Получаем валидную сессию
		cookie, accountKey, err := i.session.GetSession()
		if err != nil {
			return nil, fmt.Errorf("не удалось получить сессию (возможно все в лимите): %w", err)
		}

		req, shouldRetry, err := i.attemptCapture(ctx, useragent, cookie, accountKey, captured)
		if err != nil {
			return nil, err
		}
		if req != nil {
			return req, nil
		}
		if !shouldRetry {
			return nil, fmt.Errorf("не удалось перехватить запрос")
		}

		i.log.Info("Попытка завершена, перехожу к следующей итерации", "attempt", attempt)
	}

	return nil, fmt.Errorf("не удалось перехватить запрос после %d попыток", maxRetries)
}

//	выполняет одну попытку перехвата запроса
//
// Возвращает: (запрос, нужна_ли_повторная_попытка, ошибка)
func (i *Interceptor) attemptCapture(
	ctx context.Context,
	useragent string,
	cookie []playwright.OptionalCookie,
	accountKey string,
	captured chan *CapturedRequest,
) (*CapturedRequest, bool, error) {
	// Создаем дочерний контекст с таймаутом для операций браузера
	browserOpCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	// Создаем контекст браузера
	browserCtx, err := i.session.Browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String(useragent),
	})
	if err != nil {
		return nil, false, fmt.Errorf("не удалось создать контекст браузера: %w", err)
	}
	defer browserCtx.Close()

	// Устанавливаем куки
	if err := browserCtx.AddCookies(cookie); err != nil {
		return nil, false, fmt.Errorf("не удалось добавить куки: %w", err)
	}

	// Создаем страницу
	page, err := browserCtx.NewPage()
	if err != nil {
		return nil, false, fmt.Errorf("не удалось создать новую страницу: %w", err)
	}
	defer page.Close()

	// Устанавливаем обработчик перехвата запросов
	i.setupRequestInterceptor(page, cookie, captured)

	// Переходим на grok.com
	if _, err = page.Goto("https://grok.com", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		i.log.Error("ошибка перехода на grok.com, пробуем следующий аккаунт", "err", err)

		// Повторяем попытку
		return nil, true, nil
	}

	// Проверяем, не попали ли на страницу логина
	currentURL := page.URL()
	if strings.Contains(currentURL, "sign-in") || strings.Contains(currentURL, "login") {
		i.log.Warn("Попали на страницу логина, сессия невалидна", "account", accountKey)

		// Повторяем попытку с другим аккаунтом
		i.session.MarkInvalid(accountKey, 24*time.Hour)
		return nil, true, nil
	}

	i.log.Info("Авторизация через куки прошла успешно")

	// Отправляем первое сообщение и ждем ответа
	if err := i.sendMessageAndWait(browserOpCtx, page, "Hey grok"); err != nil {
		if strings.Contains(err.Error(), LimitMsg) || strings.Contains(err.Error(), HighDemandMsg) {
			waitTime := parseWaitTime(err.Error())
			i.log.Warn("Обнаружен лимит сообщений, меняем аккаунт", "account", accountKey, "wait", waitTime.String())

			// Повторяем попытку c другим аккаунтом
			i.session.MarkInvalid(accountKey, waitTime)
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("не удалось отправить первое сообщение: %w", err)
	}

	time.Sleep(1 * time.Second)

	// Отправляем второе сообщение (не ждем ответа, перехватываем запрос)
	if err := i.sendMessageSimple(page, "LOL"); err != nil {
		return nil, false, fmt.Errorf("не удалось отправить второе сообщение: %w", err)
	}

	i.log.Info("Ожидаем перехват /responses для второго сообщения...")

	// Ждем перехваченный запрос
	select {
	case req := <-captured:
		i.log.Info("Запрос успешно перехвачен", "url", req.URL)
		return req, false, nil
	case <-browserOpCtx.Done():
		return nil, false, browserOpCtx.Err()
	case <-time.After(30 * time.Second):
		i.log.Warn("таймаут ожидания запроса, пробуем другой аккаунт")
		return nil, true, nil
	}
}

// устанавливает обработчик перехвата запросов
func (i *Interceptor) setupRequestInterceptor(
	page playwright.Page,
	cookie []playwright.OptionalCookie,
	captured chan *CapturedRequest,
) {
	page.On("request", func(request playwright.Request) {
		if !strings.HasSuffix(request.URL(), "/responses") {
			return
		}

		i.log.Info("Найден нужный запрос", "method", request.Method(), "url", request.URL())

		// Запускаем обработку в отдельной горутине
		// чтобы не блокировать Event Loop Playwright!
		go i.processInterceptedRequest(request, cookie, captured)
	})
}

// обрабатывает перехваченный запрос в отдельной горутине
func (i *Interceptor) processInterceptedRequest(
	request playwright.Request,
	cookie []playwright.OptionalCookie,
	captured chan *CapturedRequest) {
	body, _ := request.PostData()
	headers := request.Headers()

	// Проверяем наличие куки в заголовках
	cookieHeader := ""
	if c, ok := headers["cookie"]; ok {
		cookieHeader = c
	}

	// Если куки нет, добавляем из сессии
	if cookieHeader == "" {
		var cookieParts []string
		for _, c := range cookie {
			cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", c.Name, c.Value))
		}
		cookieHeader = strings.Join(cookieParts, ";")
		headers["cookie"] = cookieHeader
		i.log.Info("куки добавлены вручную из сессии")
	}

	req := &CapturedRequest{
		Headers: headers,
		Body:    body,
		URL:     request.URL(),
	}

	// Отправляем в канал (блокирующая операция, но мы в горутине)
	captured <- req
	i.log.Info("Запрос успешно отправлен в канал")
}

// отправляет сообщение и ждет ответа Grok
func (i *Interceptor) sendMessageAndWait(ctx context.Context, page playwright.Page, msg string) error {
	// Находим поле ввода
	textarea := i.findTextarea(page)
	if textarea == nil {
		return fmt.Errorf("не удалось найти поле ввода")
	}

	// Вводим сообщение
	if err := textarea.Click(); err != nil {
		return fmt.Errorf("клик на поле ввода: %w", err)
	}
	time.Sleep(400 * time.Millisecond)

	if err := textarea.Fill(msg); err != nil {
		return fmt.Errorf("ввод сообщения: %w", err)
	}
	time.Sleep(400 * time.Millisecond)

	i.log.Info("Отправляю сообщение", "msg", msg)

	// Отправляем сообщение
	sendBtn := page.Locator("button[aria-label*='send' i]")
	count, _ := sendBtn.Count()
	if count > 0 {
		if err := sendBtn.First().Click(); err != nil {
			return fmt.Errorf("клик на кнопку отправки: %w", err)
		}
	} else {
		if err := page.Keyboard().Press("Enter"); err != nil {
			return fmt.Errorf("нажатие Enter: %w", err)
		}
	}

	// Ждем завершения ответа Grok
	return i.waitForGrokResponse(ctx, page, msg)
}

// отправляет сообщение без ожидания ответа
func (i *Interceptor) sendMessageSimple(page playwright.Page, msg string) error {
	textarea := i.findTextarea(page)
	if textarea == nil {
		return fmt.Errorf("не удалось найти поле ввода")
	}

	if err := textarea.Click(); err != nil {
		return fmt.Errorf("клик на поле ввода: %w", err)
	}
	time.Sleep(400 * time.Millisecond)

	if err := textarea.Fill(msg); err != nil {
		return fmt.Errorf("ввод сообщения: %w", err)
	}
	time.Sleep(400 * time.Millisecond)

	i.log.Info("Отправляю сообщение", "msg", msg)

	sendBtn := page.Locator("button[aria-label*='send' i]")
	count, _ := sendBtn.Count()
	if count > 0 {
		if err := sendBtn.First().Click(); err != nil {
			return fmt.Errorf("клик на кнопку отправки: %w", err)
		}
	} else {
		if err := page.Keyboard().Press("Enter"); err != nil {
			return fmt.Errorf("нажатие Enter: %w", err)
		}
	}

	time.Sleep(500 * time.Millisecond)
	return nil
}

// находит поле ввода сообщения
func (i *Interceptor) findTextarea(page playwright.Page) playwright.Locator {
	selectors := []string{
		"textarea[placeholder*='message' i]",
		"textarea[placeholder*='Ask' i]",
		"textarea[placeholder*='Grok' i]",
		"div[contenteditable='true']",
		"textarea",
	}

	for _, sel := range selectors {
		loc := page.Locator(sel)
		count, err := loc.Count()
		if err == nil && count > 0 {
			return loc.First()
		}
	}

	return nil
}

// ждет завершения ответа Grok
func (i *Interceptor) waitForGrokResponse(ctx context.Context, page playwright.Page, label string) error {
	i.log.Info("Ожидаю завершения ответа Grok", "label", label)

	// Небольшая пауза для начала генерации
	time.Sleep(3 * time.Second)

	// Проверяем наличие кнопки Stop (Grok генерирует ответ)
	stopSelectors := []string{
		"button[aria-label*='stop' i]",
		"button[aria-label*='Stop' i]",
		"button:has-text('Stop')",
		"[data-testid*='stop']",
	}

	// Ждем завершения генерации (исчезновения кнопки Stop)
	for j := 0; j < 120; j++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		generating := false
		for _, sel := range stopSelectors {
			count, err := page.Locator(sel).Count()
			if err == nil && count > 0 {
				generating = true
				break
			}
		}

		if !generating {
			i.log.Info("Grok закончил генерацию ответа", "label", label)
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("таймаут ожидания ответа Grok")
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

// количество сессий в кэше
func (i *Interceptor) Length() int {
	return i.session.Length()
}
