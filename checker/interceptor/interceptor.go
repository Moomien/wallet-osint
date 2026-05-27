package interceptor

import (
	"arkham_checker/checker/session"
	"fmt"

	"github.com/playwright-community/playwright-go"
)

//работать будет так:
//стартуется playwright браузер  -> парсятся валидные куки -> эти куки передаются в playwright instanse -> переход по loginURL -> ввод в чат двух сообщений -> перехват responses на второе сообщение

const (
	msg1 = "Hello"
	msg2 = "Hmmm"
)

type CapturedRequest struct {
	Headers map[string]string
	Cookies string
	Body    string
}

func Capture(s *session.CacheSession) (*CapturedRequest, error) {
	cookie, err := s.GetSession()
	if err != nil {
		return nil, err
	}

	ctx, err := s.Browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	})
	if err != nil {
		return nil, fmt.Errorf("создание нового контекста в перехватчике: %w", err)
	}
	defer ctx.Close()

	if err := ctx.AddCookies(cookie); err != nil {
		return nil, fmt.Errorf("добавление куки в контекст: %w", err)
	}

	page, err := ctx.NewPage()
	if err != nil {
		return nil, fmt.Errorf("новая страница: %w", err)
	}

	//TODO: реализация чека запросов браузера
	page.On("request")
}
