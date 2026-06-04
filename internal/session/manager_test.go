package session

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
)

const ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

func TestManager(t *testing.T) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	sessManager, err := NewCache(false, "grok.com")
	if err != nil {
		t.Fatal(err)
	}
	defer sessManager.Close()

	var wg sync.WaitGroup
	for range 1 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sessManager.CheckSession(ctx, ua)
		}()
	}
	wg.Wait()

	t.Log("Горутины успешно выполнены!")
	cookie, _, err := sessManager.GetSession()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(cookie)
}
