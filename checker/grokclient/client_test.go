package grokclient

import (
	"arkham_checker/checker/interceptor"
	"arkham_checker/checker/session"
	"context"
	_ "embed"
	"os"
	"os/signal"
	"syscall"
	"testing"

	"github.com/joho/godotenv"
)

//go:embed prompt.txt
var (
	prompt    string
	useragent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

func TestClient(t *testing.T) {
	if err := godotenv.Load(); err != nil {
		t.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cache, err := session.NewCache(false, "grok.com")
	if err != nil {
		t.Fatal(err)
	}

	interceptor, err := interceptor.NewInterceptor(cache)
	if err != nil {
		t.Fatal(err)
	}

	gc, err := NewGrokClient(prompt, useragent)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig("config.json", interceptor)
	if err != nil {
		t.Fatal(err)
	}

	user := "ElonMusk"
	t.Log("Отправляю сообщение c грок клиента")
	pasteURL, err := gc.SendMessage(ctx, cfg, user, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(pasteURL)
}
