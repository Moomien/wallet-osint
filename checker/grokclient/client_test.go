package grokclient

import (
	paste "arkham_checker/checker/grokclient/paste_service"
	"arkham_checker/checker/interceptor"
	"arkham_checker/checker/session"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func loadTwitterUsers(filepath string) ([]string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	users := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "@")
		if line != "" {
			users = append(users, line)
		}
	}

	return users, nil
}

// интеграционный тест отправки запросов к гроку
// и сохранения паст
func TestProcessTwitterUsers(t *testing.T) {
	if err := godotenv.Load(); err != nil {
		t.Fatalf("загрузка env: %v", err)
	}
	cache, err := session.NewCache(true, "grok.com")
	if err != nil {
		t.Fatal(err)
	}
	interceptor, err := interceptor.NewInterceptor(cache)
	if err != nil {
		t.Fatal(err)
	}

	config, err := NewGrokConfig("./sessions", interceptor)
	if err != nil {
		t.Fatalf("Ошибка загрузки сессий: %v", err)
	}
	defer config.Close()
	fmt.Printf("Загружено сессий: %d\n", len(config.GetSessionNames()))

	//загружаем из текстовика для теста
	users, err := loadTwitterUsers("./twitter_users.txt")
	if err != nil {
		t.Fatalf("Ошибка загрузки юзеров: %v", err)
	}
	fmt.Printf("Загружено юзеров: %d\n", len(users))

	promptData, err := os.ReadFile("./prompt.txt")
	if err != nil {
		t.Fatalf("Ошибка загрузки prompt.txt: %v", err)
	}
	prompt := string(promptData)
	fmt.Printf("Промпт загружен (%d символов)\n", len(prompt))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool := NewSessionPool(config)
	useragent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

	var resultsMu sync.Mutex
	results := make([]string, 0, len(users))
	var lostUsers []string
	var lostMu sync.Mutex

	startTime := time.Now()

	err = pool.RunWorkers(ctx, users, func(username, result string, err error) {
		if err != nil {
			log.Printf("[@%s] Ошибка: %v", username, err)
			lostMu.Lock()
			lostUsers = append(lostUsers, username)
			lostMu.Unlock()
			return
		}

		resultsMu.Lock()
		results = append(results, result)
		resultsMu.Unlock()

		log.Printf("[@%s] обработан", username)
	}, prompt, useragent)

	if err != nil {
		t.Fatalf("Ошибка воркеров: %v", err)
	}

	//статистика
	duration := time.Since(startTime)
	fmt.Printf("\n=== Статистика ===\n")
	fmt.Printf("Обработано юзеров: %d/%d\n", len(results), len(users))
	if len(lostUsers) > 0 {
		fmt.Printf("Потерянные юзеры: %d (%v)\n", len(lostUsers), lostUsers)
	}
	fmt.Printf("Время выполнения: %s\n", duration)
	fmt.Printf("Скорость: %.2f юзеров/сек\n", float64(len(results))/duration.Seconds())

	if len(results) > 0 {
		pasteService, err := paste.NewNotionClient()
		if err != nil {
			log.Printf("Ошибка создания pastebin сервиса: %v", err)
			return
		}
		defer pasteService.Close()

		urls := make(chan string)
		var wg sync.WaitGroup

		for _, result := range results {
			wg.Add(1)
			go func(r string) {
				defer wg.Done()
				if err := pasteService.CreatePaste(r, urls); err != nil {
					log.Print(err)
				}
			}(result)
		}

		go func() {
			wg.Wait()
			close(urls)
		}()

		savedURLs := []string{}
		for url := range urls {
			savedURLs = append(savedURLs, url)
			fmt.Printf("Паст сохранён: %s\n", url)
		}

		fmt.Printf("\nВсего сохранено паст: %d\n", len(savedURLs))
	}
}
