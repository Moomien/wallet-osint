package session

import (
	log "arkham_checker/checker/logger"
	"fmt"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

func TestManager(t *testing.T) {
	logger, err := log.NewLogger("session")
	if err != nil {
		t.Fatal(err)
	}
	cookies, err := cookieS(logger.Log)
	if err != nil {
		t.Fatal(err)
	}
	data := make(map[string][]playwright.OptionalCookie)
	for i, cook := range cookies {
		fmt.Println("Внешний цикл", i)
		var pwCookies []playwright.OptionalCookie
		for i, c := range cook {
			pwCookies = append(pwCookies, playwright.OptionalCookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   playwright.String("grok.com"),
				Path:     playwright.String("/"),
				Secure:   playwright.Bool(true),
				HttpOnly: playwright.Bool(false),
				SameSite: playwright.SameSiteAttributeLax,
			})
			fmt.Println("внутренний цикл", i)
			fmt.Println(c)
		}
		data["test"] = pwCookies
	}
	pw, _ := playwright.Run()
	browser, _ := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(false)})
	ctx, _ := browser.NewContext()
	cookie := data["test"]
	if err := ctx.AddCookies(cookie); err != nil {
		t.Fatal(err)
	}
	page, _ := ctx.NewPage()
	_, err = page.Goto("https://grok.com",
		playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
		})
	if err != nil {
		t.Fatal("HE получилось зайти на страницу grok.com")
	}
	time.Sleep(time.Minute)
}
