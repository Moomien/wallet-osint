package session

import (
	log "arkham_checker/checker/logger"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
)

const (
	selectorChatInput = "div[contenteditable='true']"
	selectorLimitMsg  = "#last-reply-container >> text=Достигнут лимит сообщений"
	testCheckMessage  = "Heeey grok testing you just and so"
	limitCheckTimeout = 4000
)

// CacheSession управляет сессиями Playwright и их кэшированием.
type CacheSession struct {
	mu        sync.RWMutex
	data      map[string]*SessionInfo
	cachePath string
	domain    string
	pw        *playwright.Playwright
	Browser   playwright.Browser
	log       *log.Logger
}

// SessionInfo содержит данные о состоянии конкретной сессии.
type SessionInfo struct {
	Cookies     []playwright.OptionalCookie `json:"cookies"`
	Valid       bool                        `json:"Valid"`
	LastChecked time.Time                   `json:"LastChecked"`
	Checking    bool                        `json:"-"`
	ResetAt     time.Time                   `json:"ResetAt,omitempty"` // для ротации сессий
}

type Cookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// NewCache создает новый экземпляр менеджера сессий и инициализирует браузер.
// собирает один единый json файл из множества других json
// запускает чек куки и проходится по каждому куки из json и
// аллоцирует куки в мапу
func NewCache(headless bool, domain string) (*CacheSession, error) {
	//делаем отдельный логгер для этого модуля
	baseDir := "session"
	cachePath := filepath.Join(baseDir, "cache.json")
	logger, err := log.NewLogger(baseDir)
	if err != nil {
		return nil, fmt.Errorf("создание логгера: %w", err)
	}

	cookies, err := cookieS(logger.Log, cachePath)
	if err != nil {
		return nil, fmt.Errorf("загрузка куки: %w", err)
	}

	data := make(map[string]*SessionInfo)
	// Сначала пытаемся загрузить существующий кэш
	if bs, err := os.ReadFile(cachePath); err == nil {
		_ = json.Unmarshal(bs, &data)
	}

	for i, accountCookies := range cookies {
		key := fmt.Sprintf("Account_%d", i+1)

		// Если сессия уже есть в кэше, не перезаписываем её статус
		if _, exists := data[key]; exists {
			continue
		}

		var pwCookies []playwright.OptionalCookie
		for _, c := range accountCookies {
			pwCookies = append(pwCookies, playwright.OptionalCookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   playwright.String(domain),
				Path:     playwright.String("/"),
				Secure:   playwright.Bool(true),
				HttpOnly: playwright.Bool(false),
				SameSite: playwright.SameSiteAttributeLax,
			})
		}
		data[key] = &SessionInfo{
			Cookies:     pwCookies,
			Valid:       true,
			LastChecked: time.Now(),
		}
	}

	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("запуск playwright: %w", err)
	}

	success := false
	defer func() {
		if !success {
			_ = pw.Stop()
		}
	}()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
	})
	if err != nil {
		return nil, fmt.Errorf("запуск браузера chromium: %w", err)
	}
	defer func() {
		if !success {
			_ = browser.Close()
		}
	}()

	c := &CacheSession{
		data:      data,
		cachePath: cachePath,
		domain:    domain,
		pw:        pw,
		Browser:   browser,
		log:       logger,
	}
	if err = c.cacheJSON(); err != nil {
		return nil, fmt.Errorf("инициализация cache.json: %w", err)
	}

	success = true
	return c, nil
}

// GetSession возвращает первую доступную валидную куку из кэша.
func (c *CacheSession) GetSession() ([]playwright.OptionalCookie, string, error) {
	c.mu.Lock()
	var targetCookies []playwright.OptionalCookie
	var targetKey string
	found := false

	for key, session := range c.data {
		if !session.Valid && !session.ResetAt.IsZero() && time.Now().After(session.ResetAt) && !session.Checking {
			session.Valid = true
			session.ResetAt = time.Time{}
			c.log.Info("Лимит аккаунта истек, вовращаем в строй", "account", key)
		}

		if !session.Valid {
			continue
		}

		targetCookies = session.Cookies
		targetKey = key
		found = true
		break
	}
	c.mu.Unlock()

	if !found {
		return nil, "", errors.New("валидная сессия не найдена")
	}

	return targetCookies, targetKey, nil
}

// MarkInvalid помечает сессию как невалидную
// например, при достижении лимита
func (c *CacheSession) MarkInvalid(key string, waitTime time.Duration) {
	c.mu.Lock()
	session, ok := c.data[key]
	if ok {
		session.Valid = false
		session.LastChecked = time.Now()
		if waitTime > 0 {
			session.ResetAt = time.Now().Add(waitTime)
			c.log.Info("Mark: сессия в лимите", "account", key, "reset_at",
				session.ResetAt.Format("15:04:05"))
		} else {
			c.log.Info("Что то не то с лимитом на аккаунте", "account", key)
			session.ResetAt = time.Now().Add(time.Hour * 24)
		}
	}
	c.mu.Unlock()
	if ok {
		if err := c.cacheJSON(); err != nil {
			c.log.Error("обновление файла cache.json", "err", err)
		}
	}
}

// сохраняет мапу в cache.json
func (c *CacheSession) cacheJSON() error {
	c.mu.RLock()
	prettyJSON, err := json.MarshalIndent(c.data, "", "  ")
	c.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	return os.WriteFile(c.cachePath, prettyJSON, 0644)
}

// CheckSession проверяет валидность сессии, выполняя тестовое действие в браузере.
func (c *CacheSession) CheckSession(ctx context.Context, useragent string) {
	select {
	case <-ctx.Done():
		return
	default:
	}
	c.mu.Lock()
	var targetKey string
	var oldest time.Time
	found := false

	for key, session := range c.data {
		if session.Checking || !session.Valid {
			continue
		}
		if !found || session.LastChecked.Before(oldest) {
			oldest = session.LastChecked
			targetKey = key
			found = true
		}
	}

	if !found {
		c.mu.Unlock()
		c.log.Info("Нет доступных аккаунтов для проверки")
		return
	}

	session := c.data[targetKey]
	session.Checking = true
	cookie := session.Cookies
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		session.Checking = false
		c.mu.Unlock()
	}()

	//создаем изолированную сессию в браузере
	browserCtx, err := c.Browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String(useragent),
	})
	if err != nil {
		c.log.Error("HE удалось создать новый контекст браузера", "err", err)
		return
	}
	c.log.Info("Создал браузер")
	defer browserCtx.Close()

	// Горутина для мгновенной отмены при Ctrl+C
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			browserCtx.Close()
		case <-done:
		}
	}()
	defer close(done)

	//добавляем куки в контекст
	if err = browserCtx.AddCookies(cookie); err != nil {
		c.log.Error("не удалось добавить куки в контекст", "err", err)
		c.mu.Lock()
		if session, ok := c.data[targetKey]; ok {
			session.Valid = false
			session.LastChecked = time.Now()
			if err := c.cacheJSON(); err != nil {
				c.log.Error("критическая ошибка: не удалось обновить cache.json после сбоя кук", "err", err)
			}
		}
		c.mu.Unlock()
		return
	}

	//новая страница
	page, err := browserCtx.NewPage()
	if err != nil {
		c.log.Error("HE удалось создать новую страницу в браузере", "err", err)
		return
	}
	targetURL := fmt.Sprintf("https://%s", c.domain)
	//переход на домен
	_, err = page.Goto(targetURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	if err != nil {
		c.log.Warn("HE получилось зайти на целевую страницу", "err", err)
		return
	}

	c.log.Info("Ввожу текст в чат грока")
	textarea := page.Locator(selectorChatInput).First()
	if err = textarea.Click(); err != nil {
		c.log.Error("HE удалось найти нужный селектор для ввода в чат грок", "err", err)
		return
	}

	c.log.Info("клик на поле ввода")
	if err = textarea.Fill(testCheckMessage); err != nil {
		c.log.Error("HE получилось ввести сообщение", "err", err)
		return
	}

	//отправка сообщения
	c.log.Info("Ввод текста")
	if err = page.Keyboard().Press("Enter"); err != nil {
		c.log.Error("HE удалось отправить текст", "err", err)
		return
	}
	c.log.Info("Отправил текст")
	//ждем ответа
	_ = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})

	//ищем селектор лимита
	limitLocator := page.Locator(selectorLimitMsg)
	err = limitLocator.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(limitCheckTimeout),
	})

	c.mu.Lock()
	if err == nil {
		c.log.Info("АККАУНТ B ЛИМИТЕ: Найдено сообщение в контейнере ответа!", "session_key", targetKey)
		session.Valid = false
	} else {
		c.log.Info("Лимита нет, всё ок.", "session_key", targetKey)
		session.Valid = true
	}
	session.LastChecked = time.Now()
	c.mu.Unlock()

	//обновление в cache.json
	if err = c.cacheJSON(); err != nil {
		c.log.Error("HE получилось обновить данные в cache.json", "err", err)
		return
	}
}

// Close корректно завершает работу браузера и Playwright и логгера
func (c *CacheSession) Close() error {
	var errs []string
	if c.pw != nil {
		if err := c.pw.Stop(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if c.Browser != nil {
		if err := c.Browser.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if c.log != nil {
		if err := c.log.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("Errors during close:%s", strings.Join(errs, "; "))
	}

	return nil
}

// собирает куки из session/cookies и
// возвращает слайс куки, которые потом пишем в мапу для playwright
func cookieS(log *slog.Logger, cachePath string) ([][]Cookie, error) {
	dir := filepath.Dir(cachePath)
	cookiesPath := filepath.Join(dir, "cookies")
	files, err := os.ReadDir(cookiesPath)
	if err != nil {
		return nil, fmt.Errorf("чтение директории куки %s: %w", dir, err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("папка %s пустая, добавьте JSON файлы с куками", dir)
	}

	var allCookies [][]Cookie
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		fullpath := filepath.Join(cookiesPath, file.Name())
		bs, err := os.ReadFile(fullpath)
		if err != nil {
			log.Warn("пропуск файла: ошибка чтения", "file", file.Name(), "err", err)
			continue
		}

		if len(bs) == 0 {
			log.Warn("пропуск файла: пустой", "file", file.Name())
			continue
		}

		str := strings.TrimSpace(string(bs))
		if !strings.HasPrefix(str, "[") {
			str = "[" + strings.TrimSuffix(str, ",") + "]"
		}

		var cookie []Cookie
		if err := json.Unmarshal([]byte(str), &cookie); err != nil {
			log.Warn("пропуск файла: поврежденный JSON", "file", file.Name(), "err", err)
			continue
		}
		allCookies = append(allCookies, cookie)
	}
	return allCookies, nil
}

// количество сессий
func (c *CacheSession) Length() int {
	return len(c.data)
}
