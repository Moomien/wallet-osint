package session

import (
	log "arkham_checker/checker/logger"
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

type CacheSession struct {
	mu      sync.RWMutex
	data    map[string]*SessionInfo
	pw      *playwright.Playwright
	browser playwright.Browser
	Logger  *log.Logger
}

type SessionInfo struct {
	Cookies     []playwright.OptionalCookie `json:"cookies"`
	Valid       bool                        `json:"Valid"`
	LastChecked time.Time                   `json:"LastChecked"`
	Checking    bool                        `json:"-"`
}

type Cookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// собирает один единый json файл из множества других json
// запускает чек куки и проходится по каждому куки из json и
// аллоцирует куки в мапу
func NewCache() (*CacheSession, error) {
	//делаем отдельный логгер для этого модуля
	logger, err := log.NewLogger("session")
	if err != nil {
		return nil, fmt.Errorf("создание логгера: %w", err)
	}

	cookies, err := cookieS(logger.Log)
	if err != nil {
		return nil, fmt.Errorf("загрузка куки: %w", err)
	}

	data := make(map[string]*SessionInfo)
	for i, accountCookies := range cookies {
		key := fmt.Sprintf("Account_%d", i+1)
		var pwCookies []playwright.OptionalCookie
		for _, c := range accountCookies {
			pwCookies = append(pwCookies, playwright.OptionalCookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   playwright.String("grok.com"),
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

	if err = cacheJSON(data); err != nil {
		return nil, fmt.Errorf("инициализация cache.json: %w", err)
	}

	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("запуск playwright: %w", err)
	}
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		_ = pw.Stop()
		return nil, fmt.Errorf("запуск браузера chromium: %w", err)
	}
	return &CacheSession{
		data:    data,
		pw:      pw,
		browser: browser,
		Logger:  logger,
	}, nil
}

// проходится по кэшу и возвращает первые найденные куки из кэша
func (c *CacheSession) GetSession() ([]playwright.OptionalCookie, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, session := range c.data {
		if !session.Valid {
			continue
		}
		session.LastChecked = time.Now()
		if err := cacheJSON(c.data); err != nil {
			return nil, fmt.Errorf("обновление кэша: %w", err)
		}
		return session.Cookies, nil
	}

	return nil, errors.New("валидная сессия не найдена")
}

// берет первую валидную куку из кэша, заходит на аккаунт и проверяет валидность сессии
func (c *CacheSession) CheckSession() {
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
		c.Logger.Info("Нет доступных аккаунтов для проверки")
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
	ctx, err := c.browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	})
	if err != nil {
		c.Logger.Error("HE удалось создать новый контекст браузера", "err", err)
		return
	}
	c.Logger.Info("Создал браузер")
	defer ctx.Close()

	//добавляем куки в контекст
	if err = ctx.AddCookies(cookie); err != nil {
		c.Logger.Error("не удалось добавить куки в контекст", "err", err)
		c.mu.Lock()
		if session, ok := c.data[targetKey]; ok {
			session.Valid = false
			session.LastChecked = time.Now()
			if err := cacheJSON(c.data); err != nil {
				c.Logger.Error("критическая ошибка: не удалось обновить cache.json после сбоя кук", "err", err)
			}
		}
		c.mu.Unlock()
		return
	}

	//новая страница
	page, err := ctx.NewPage()
	if err != nil {
		c.Logger.Error("HE удалось создать новую страницу в браузере", "err", err)
		return
	}
	//переход в чат грока
	_, err = page.Goto("https://grok.com",
		playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
		})
	if err != nil {
		c.Logger.Warn("HE получилось зайти на страницу grok.com", "err", err)
		return
	}

	c.Logger.Info("Ввожу текст в чат грока")
	textarea := page.Locator("div[contenteditable='true']").First()
	if err = textarea.Click(); err != nil {
		c.Logger.Error("HE удалось найти нужный селектор для ввода в чат грок", "err", err)
		return
	}

	c.Logger.Info("клик на поле ввода")
	if err = textarea.Fill("Heeey grok testing you just and so"); err != nil {
		c.Logger.Error("HE получилось ввести сообщение", "err", err)
		return
	}

	//отправка сообщения
	c.Logger.Info("Ввод текста")
	if err = page.Keyboard().Press("Enter"); err != nil {
		c.Logger.Error("HE удалось отправить текст", "err", err)
		return
	}
	c.Logger.Info("Отправил текст")
	time.Sleep(500 * time.Millisecond)

	//ищем селектор лимита до 4 секунд
	limitLocator := page.Locator("#last-reply-container >> text=Достигнут лимит сообщений")
	err = limitLocator.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10000),
	})

	if err == nil {
		c.Logger.Info("АККАУНТ B ЛИМИТЕ: Найдено сообщение в контейнере ответа!", "session_key", targetKey)
		session.Valid = false
	} else {
		c.Logger.Info("Лимита нет, всё ок.", "session_key", targetKey)
		session.Valid = true
	}
	session.LastChecked = time.Now()
	//обновление в cache.json
	if err = cacheJSON(c.data); err != nil {
		c.Logger.Error("HE получилось обновить данные в cache.json", "err", err)
		return
	}
}

// закрытие pw и драйвера
func (c *CacheSession) Close() error {
	var errs []string
	if c.pw != nil {
		if err := c.pw.Stop(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if c.browser != nil {
		if err := c.browser.Close(); err != nil {
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
func cookieS(log *slog.Logger) ([][]Cookie, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("получение рабочей директории: %w", err)
	}

	if strings.Contains(dir, "session") {
		dir = filepath.Dir(dir)
	}
	dir = filepath.Join(dir, "session/cookies")

	files, err := os.ReadDir(dir)
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

		fullpath := filepath.Join(dir, file.Name())
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

// сохраняет мапу в cache.json
func cacheJSON(data map[string]*SessionInfo) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("рабочая директория: %w", err)
	}
	if strings.Contains(dir, "session") {
		dir = filepath.Dir(dir)
	}
	cachepath := filepath.Join(dir, "session/cache.json")
	prettyJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	if err := os.WriteFile(cachepath, prettyJSON, 0644); err != nil {
		return fmt.Errorf("запись файла %s: %w", cachepath, err)
	}
	return nil
}
