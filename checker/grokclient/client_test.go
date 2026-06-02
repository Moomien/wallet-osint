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

	// 2. Загружаем твиттер юзеров
	users, err := loadTwitterUsers("./twitter_users.txt")
	if err != nil {
		t.Fatalf("Ошибка загрузки юзеров: %v", err)
	}
	fmt.Printf("Загружено юзеров: %d\n", len(users))

	// 3. Загружаем промпт из файла
	promptData, err := os.ReadFile("./prompt.txt")
	if err != nil {
		t.Fatalf("Ошибка загрузки prompt.txt: %v", err)
	}
	prompt := string(promptData)
	fmt.Printf("Промпт загружен (%d символов)\n", len(prompt))

	// 4. Создаём контекст с отменой по Ctrl+C
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 5. Создаём пул и запускаем воркеры
	pool := NewSessionPool(config)
	useragent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

	// 6. Создаём канал задач
	taskQueue := make(chan string, len(users))
	for _, user := range users {
		taskQueue <- user
	}
	close(taskQueue)

	// 7. Список для результатов (потокобезопасный)
	var resultsMu sync.Mutex
	results := make([]string, 0, len(users))

	// Счётчик потерянных юзеров
	var lostUsers []string
	var lostMu sync.Mutex

	// 8. Запускаем параллельные воркеры
	startTime := time.Now()

	err = pool.RunWorkers(ctx, taskQueue, func(ctx context.Context, client *GrokClient, sessionName string, tasks <-chan string) error {
		log.Printf("[%s] Воркер запущен", sessionName)

		for username := range tasks {
			// Проверяем контекст перед каждым запросом
			select {
			case <-ctx.Done():
				log.Printf("[%s] Воркер остановлен по Ctrl+C", sessionName)
				return ctx.Err()
			default:
			}

			// Пытаемся обработать с retry
			maxRetries := 3
			var response string
			var lastErr error

			for attempt := 0; attempt < maxRetries; attempt++ {
				response, lastErr = client.SendMessage(ctx, config, username, 0)

				if lastErr == nil {
					// Успех!
					break
				}

				// Проверяем, это 429 ошибка?
				if grokErr, ok := lastErr.(*GrokError); ok && grokErr.StatusCode == 429 {
					retryAfter := grokErr.RetryAfter
					if retryAfter == 0 {
						retryAfter = 5 * time.Second
					}

					log.Printf("[%s] 429 для @%s (попытка %d/%d), ждём %v",
						sessionName, username, attempt+1, maxRetries, retryAfter)

					// Ждём и пробуем снова
					select {
					case <-time.After(retryAfter):
						continue
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				// Для других ошибок - не retry
				break
			}

			if lastErr != nil {
				log.Printf("[%s] Не удалось обработать @%s после %d попыток: %v",
					sessionName, username, maxRetries, lastErr)
				lostMu.Lock()
				lostUsers = append(lostUsers, username)
				lostMu.Unlock()
				continue
			}

			// Сохраняем результат
			resultsMu.Lock()
			results = append(results, response)
			resultsMu.Unlock()

			log.Printf("[%s] @%s обработан", sessionName, username)
		}

		log.Printf("[%s] Воркер завершён", sessionName)
		return nil
	}, prompt, useragent)

	if err != nil {
		t.Fatalf("Ошибка воркеров: %v", err)
	}

	// 9. Выводим статистику
	duration := time.Since(startTime)
	fmt.Printf("\n=== Статистика ===\n")
	fmt.Printf("Обработано юзеров: %d/%d\n", len(results), len(users))
	if len(lostUsers) > 0 {
		fmt.Printf("Потерянные юзеры: %d (%v)\n", len(lostUsers), lostUsers)
	}
	fmt.Printf("Время выполнения: %s\n", duration)
	fmt.Printf("Скорость: %.2f юзеров/сек\n", float64(len(results))/duration.Seconds())

	// 10. Сохраняем результаты в pastebin
	if len(results) > 0 {
		fmt.Println("\n=== Сохранение в Pastebin ===")

		// Создаём pastebin сервис
		pasteService, err := paste.NewPastebin()
		if err != nil {
			log.Printf("Ошибка создания pastebin сервиса: %v", err)
			return
		}
		defer pasteService.Close()

		// Проходимся по результатам и сохраняем каждый отдельно
		for i, result := range results {
			pasteURL, err := pasteService.CreatePaste(result)
			if err != nil {
				log.Printf("Ошибка сохранения пасты %d: %v", i+1, err)
			} else {
				fmt.Printf("Паст %d сохранён: %s\n", i+1, pasteURL)
			}

			// Rate limiting для pastebin API
			time.Sleep(500 * time.Millisecond)
		}

		fmt.Printf("\nВсего сохранено паст: %d\n", len(results))
	}
}
