package grokclient

import (
	log "arkham_checker/checker/logger"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// SessionPool управляет пулом сессий для параллельной работы
type SessionPool struct {
	config        *GrokConfig
	sessions      []string // список всех сессий
	mu            sync.RWMutex
	activeWorkers atomic.Int32
	log           *log.Logger
}

// NewSessionPool создаёт новый пул сессий
func NewSessionPool(config *GrokConfig) *SessionPool {
	logger, err := log.NewLogger("SessionPool")
	if err != nil {
		fmt.Println("Ошибка создания логгера SessionPool")
		return nil
	}
	sessions := config.GetSessionNames()
	return &SessionPool{
		config:   config,
		sessions: sessions,
		log:      logger,
	}
}

// WorkerFunc - функция которую выполняет воркер
// Принимает контекст, клиент, имя сессии и общую очередь задач
type WorkerFunc func(ctx context.Context, client *GrokClient, sessionName string, taskQueue <-chan string) error

// RunWorkers запускает N горутин (по одной на каждую сессию)
// Каждая горутина берёт задачи из общей очереди taskQueue
func (sp *SessionPool) RunWorkers(ctx context.Context, taskQueue <-chan string,
	workerFunc WorkerFunc, prompt string, useragent string) error {
	if len(sp.sessions) == 0 {
		return fmt.Errorf("нет доступных сессий")
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(sp.sessions))

	sp.activeWorkers.Store(int32(len(sp.sessions)))

	// Запускаем по одному воркеру на каждую сессию
	for _, sessionName := range sp.sessions {
		wg.Add(1)
		go func(session string) {
			defer wg.Done()
			defer sp.activeWorkers.Add(-1)

			// Создаём отдельный клиент для каждой горутины
			client, err := NewGrokClient(prompt, useragent)
			if err != nil {
				errChan <- fmt.Errorf("создание клиента для сессии %s: %w", session, err)
				return
			}
			defer client.Close()

			// Устанавливаем сессию
			client.SetSession(session)

			// Выполняем функцию воркера
			if err := workerFunc(ctx, client, session, taskQueue); err != nil {
				errChan <- fmt.Errorf("воркер %s: %w", session, err)
			}
		}(sessionName)
	}

	// Ждём завершения всех воркеров
	wg.Wait()
	close(errChan)

	// Собираем ошибки
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("ошибки воркеров: %v", errors)
	}

	return nil
}
