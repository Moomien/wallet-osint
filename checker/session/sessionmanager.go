package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
)

type SessionInfo struct {
	Cookies     []playwright.OptionalCookie `json:"cookies"`
	Valid       bool                        `json:"Valid"`
	LastChecked time.Duration               `json:"LastChecked"`
}

type CacheSession struct {
	mu      sync.RWMutex
	data    map[string]*SessionInfo
	logger  *slog.Logger
	logfile *os.File
}

type Cookie struct {
	Name        string        `json:"name"`
	Value       string        `json:"value"`
	Valid       bool          `json:"Valid"`
	LastChecked time.Duration `json:"LastChecker"`
}

// собирает один единый json файл из множества других json
// запускает чек куки и проходится по каждому куки из json и
// аллоцирует куки в мапу
func NewCache(baseLogger *slog.Logger) (*CacheSession, error) {
	//делаем отдельный логгер для этого модуля
	logger, file, err := newLogger()
	if err != nil {
		logger.Warn("Не получилось создать логгер", "err", err)
	}
	cookies, err := cookieS(logger)
	if err != nil {
		logger.Error("Не удалось прочитать куки из файлов", "error", err)
		return nil, err
	}

	data := make(map[string]*SessionInfo)
	for i, c := range cookies {
		key := fmt.Sprintf("Cookie_%d", i+1)
		data[key] = &SessionInfo{
			Cookies: []playwright.OptionalCookie{
				{
					Name:     c.Name,
					Value:    c.Value,
					Domain:   playwright.String("grok.com"),
					Path:     playwright.String("/"),
					Secure:   playwright.Bool(true),
					HttpOnly: playwright.Bool(false),
					SameSite: playwright.SameSiteAttributeLax,
				}},
		}
	}
	if err := cacheJSON(data); err != nil {
		logger.Error("Не удалось сохранить все куки в один cache.json", "err", err)
		return nil, err
	}

	return &CacheSession{
		data:    data,
		logger:  logger,
		logfile: file, //обязательно вызвать чтобы закрыть
	}, nil
}

func (c *CacheSession) Close() error {

}

// проходится по кэшу и возвращает первые найденные валидный куки
func (c *CacheSession) GetSession(key string) ([]playwright.OptionalCookie, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cookie, found := c.data[key]
	if !found {
		return nil, errors.New("HE получилось найти. Ключ для куки не найден")
	}
	return cookie.Cookies, nil
}

// чек сессий раз в полчаса(?)
// что возвращает то
func (c *CacheSession) CheckSession(key string) {
	pw, err := playwright.Run()
	if err != nil {
		c.logger.Error("Ошибка запуска playwright инстанса", "err", err)
		return
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(false)})
	if err != nil {
		c.logger.Error("Ошибка запуска драйвера", "err", err)
		return
	}
	defer browser.Close()

	//создаем контекст изолированную сессию
	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	})

	if err != nil {
		c.logger.Error("HE удалось создать новый контекст браузера", "err", err)
	}
	defer ctx.Close()

	cookie, err := c.GetSession(key)
	ctx.AddCookies(cookie)
	//TODO: сделать проверку валидности куки
	page, err := ctx.NewPage()
	if err != nil {
		c.logger.Error("HE удалось создать новую страницу в браузере", "err", err)
		return
	}
	_, err = page.Goto("https://grok.com",
		playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
		})
	if err != nil {
		c.logger.Warn("HE получилось зайти на страницу grok.com", "err", err)
		return
	}
	c.logger.Info("Ввожу текст в чат грока")
	textarea := page.Locator("div[contenteditable='true']").First()
	if err := textarea.Click(); err != nil {
		c.logger.Error("HE удалось найти нужный селектор для ввода в чат грок", "err", err)
		return
	}
	fmt.Println("клик на поле ввода")
	if err := textarea.Fill("Heeey grok testing you just and so"); err != nil {
		c.logger.Error("HE получилось ввести сообщение", "err", err)
		return
	}
	fmt.Println("Ввод текста")
	if err := page.Keyboard().Press("Enter"); err != nil {
		c.logger.Error("HE удалось отправить текст", "err", err)
		return
	}
	fmt.Println("Отправил текст")
	//ищем селектор лимита и ждем 4 секунды
	limitLocator := page.Locator("#last-reply-container >> text=Достигнут лимит сообщений")
	limitLocator.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(4000),
	})

	if err == nil {
		c.logger.Info("АККАУНТ B ЛИМИТЕ: Найдено сообщение в контейнере ответа!")
		c.data[key].Valid = false
	} else {
		c.logger.Info("Лимита нет, всё ок.")
		c.data[key].Valid = true
	}

}

// возвращает слайс куки которые потом пишем в мапу для playwright
func cookieS(log *slog.Logger) ([]Cookie, error) {
	path, err := os.Executable()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(filepath.Dir(path), "session/cookies")
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errors.New("Папка session/cookies/ пустая. Закинь туда куки в формате .json!")
	}

	var allCookies []Cookie
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.Size() == 0 {
			slog.Error("Файл пустой, пропускаем", "file", file.Name())
			continue
		}

		if !strings.HasSuffix(file.Name(), ".json") {
			log.Warn("Убери из session/cookies/ не .json файлы")
			continue
		}

		fullpath := filepath.Join(filepath.Dir(path), "session/cookies/", file.Name())
		bs, err := os.ReadFile(fullpath)
		if err != nil {
			log.Warn("Ошибка чтения файла, пропускаем", "file", fullpath, "err", err)
			continue
		}

		str := strings.TrimSpace(string(bs))
		if !strings.HasPrefix(str, "[") {
			str = strings.TrimSuffix(str, ",")
			str = "[" + str + "]"
		}

		var cookie []Cookie
		err = json.Unmarshal([]byte(str), &cookie)
		if err != nil {
			log.Warn("Пропущен поврежденный JSON-файл куки", "file", file.Name(), "err", err)
			continue
		}
		allCookies = append(allCookies, cookie...)
	}
	return allCookies, nil
}

func newLogger() (*slog.Logger, *os.File, error) {
	path, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}

	logpath := filepath.Join(filepath.Dir(path), "session.log")
	file, err := os.OpenFile(logpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {

	}

	multiWriter := io.MultiWriter(os.Stdout, file)
	logger := slog.New(slog.NewJSONHandler(multiWriter, nil)).With("component", "session")

	return logger, file, nil
}

func cacheJSON(data map[string]*SessionInfo) error {
	path, err := os.Executable()
	if err != nil {
		return err
	}

	cachepath := filepath.Join(filepath.Dir(path), "session/cache.json")
	prettyJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(cachepath, prettyJSON, 0644); err != nil {
		return err
	}
	return nil
}
