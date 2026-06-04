package grokclient

import (
	log "arkham_checker/internal/logger"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// управляет пулом сессий для параллельной работы
type SessionPool struct {
	config   *GrokConfig
	sessions []string
	log      log.Log
}

// создаёт новый пул сессий
func NewSessionPool(config *GrokConfig) *SessionPool {
	logger, err := log.NewLogger("session_pool")
	if err != nil {
		fmt.Println("Ошибка создания логгера session_pool")
		return nil
	}
	sessions := config.GetSessionNames()
	return &SessionPool{
		config:   config,
		sessions: sessions,
		log:      logger,
	}
}

//	запускает воркеры для обработки пользователей
//
// с ограничением частоты запросов (1 RPS) на уровне сессий.
// Поддерживает автоматический повтор запросов при ошибке 429.
func (sp *SessionPool) RunWorkers(
	ctx context.Context,
	users []string,
	onResult func(username, result string, err error),
	prompt string,
	useragent string,
) error {
	if len(sp.sessions) == 0 {
		return fmt.Errorf("нет доступных сессий")
	}
	if len(users) == 0 {
		return fmt.Errorf("нет пользователей для обработки")
	}

	// Создаём по одному клиенту на каждую сессию
	// Каждый клиент имеет rate limiter 1 RPS внутри resty
	clients := make(map[string]*GrokClient)
	for _, sessionName := range sp.sessions {
		client, err := NewGrokClient(prompt, useragent)
		if err != nil {
			return fmt.Errorf("создание клиента для сессии %s: %w", sessionName, err)
		}
		client.SetSession(sessionName)
		clients[sessionName] = client
	}

	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	var wg sync.WaitGroup
	var sessionIndex atomic.Uint32

	for _, username := range users {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				sp.log.Info("Остановлена по контексту", "username", user)
				return
			default:
			}

			// Выбираем сессию (round-robin)
			idx := sessionIndex.Add(1) % uint32(len(sp.sessions))
			sessionName := sp.sessions[idx]

			// Берём существующий клиент для этой сессии
			client := clients[sessionName]

			sp.log.Info("Использую сессию", "username", user, "session", sessionName)

			// Пытаемся отправить с retry для 429
			maxRetries := 3
			var response string
			var lastErr error

			for attempt := 0; attempt < maxRetries; attempt++ {
				response, lastErr = client.SendMessage(ctx, sp.config, user, 0)

				if lastErr == nil {
					// Успех
					break
				}

				if grokErr, ok := lastErr.(*GrokError); ok && grokErr.StatusCode == 429 {
					retryAfter := grokErr.RetryAfter
					if retryAfter == 0 {
						retryAfter = 5 * time.Second
					}
					sp.log.Warn("429 Too Many Requests, ждём retry", "username", user, "attempt", attempt+1, "max", maxRetries, "retry_after", retryAfter)

					select {
					case <-time.After(retryAfter):
						continue
					case <-ctx.Done():
						sp.log.Info("Остановлена по контексту во время retry", "username", user)
						return
					}
				}
				break
			}

			if lastErr != nil {
				sp.log.Error("Не удалось обработать после попыток", "username", user, "max_retries", maxRetries, "error", lastErr)
				onResult(user, "", lastErr)
			} else {
				sp.log.Info("Успешно обработан", "username", user)
				onResult(user, response, nil)
			}
		}(username)
	}

	wg.Wait()
	sp.log.Info("Все горутины завершены")
	return nil
}
