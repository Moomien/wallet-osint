package grokclient

import (
	"arkham_checker/checker/interceptor"
	"arkham_checker/checker/session"
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"

	"github.com/joho/godotenv"
)

const ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

func TestClient(t *testing.T) {
	if err := godotenv.Load(); err != nil {
		t.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	gc, err := NewGrokClient()
	if err != nil {
		t.Fatal(err)
	}

	cache, err := session.NewCache(false, "grok.com")
	if err != nil {
		t.Fatal(err)
	}

	interceptor, err := interceptor.NewInterceptor(cache)
	if err != nil {
		t.Fatal(err)
	}

	request, err := interceptor.Capture(ctx, ua)
	if err != nil {
		t.Fatal(err)
	}

	user := "ElonMusk"
	t.Log("Отправляю сообщение c грок клиента")
	pasteURL, err := gc.SendMessage(ctx, request, user)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(pasteURL)

}
