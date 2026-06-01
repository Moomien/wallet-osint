package interceptor

import (
	"arkham_checker/checker/session"
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

const ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

func TestInterceptor(t *testing.T) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s, err := session.NewCache(false, "grok.com")
	if err != nil {
		t.Fatalf("создание нового кэша: %v", err)
	}
	defer s.Close()

	interceptor, err := NewInterceptor(s)
	if err != nil {
		t.Fatalf("создание нового перехватчика: %v", err)
	}
	defer interceptor.Close()

	req, err := interceptor.Capture(ctx, ua)
	if err != nil {
		t.Fatalf("захват запроса: %v", err)
	}
	t.Log("Headers:", req.Headers)
	t.Log("Body:", req.Body)
	t.Log("URL:", req.URL)
}
