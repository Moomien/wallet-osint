package checker

import (
	log "arkham_checker/internal/logger"
	"arkham_checker/internal/storage"
	"context"
	"fmt"
	"sync"

	"golang.org/x/time/rate"
)

// restyLimiter — адаптер rate.Limiter для resty (Wait вместо Allow)
type restyLimiter struct {
	limiter *rate.Limiter
}

func newRestyLimiter(rps, burst int) *restyLimiter {
	return &restyLimiter{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
}

func (r *restyLimiter) Allow() bool {
	if err := r.limiter.Wait(context.Background()); err != nil {
		return false
	}
	return true
}

// TwitterFetcher — интерфейс для получения Twitter по адресу
type TwitterFetcher interface {
	FetchTwitter(ctx context.Context, address string) (string, error)
}

// возвращает линки и остатки адресов если есть
func CollectTwitters(ctx context.Context, db storage.Storage, fetcher TwitterFetcher, addresses []string) ([]string, []string, error) {
	logger, err := log.NewLogger("checker")
	if err != nil {
		return nil, nil, fmt.Errorf("создание логгера checker: %w", err)
	}
	defer logger.Close()

	twitter := make([]string, len(addresses))
	completed := make([]bool, len(addresses))

	var wg sync.WaitGroup

	for i, adres := range addresses {
		select {
		case <-ctx.Done():
			goto wait
		default:
			wg.Add(1)
			go func(idx int, adr string) {
				defer wg.Done()

				tw, err := fetcher.FetchTwitter(ctx, adr)
				if err != nil {
					if ctx.Err() != nil {
						return // Canceled
					}
					completed[idx] = true
					logger.Info("Failed to fetch wallet", "index", idx, "wallet", adr, "error", err)
					return
				}

				if tw != "" {
					twitter[idx] = tw
					logger.Info("Fetched wallet Twitter", "index", idx, "wallet", adr, "twitter", tw)
					return
				}

				completed[idx] = true
			}(i, adres)
		}
	}

wait:
	wg.Wait()
	var rem []string
	var batch []string
	for i, done := range completed {
		if !done {
			rem = append(rem, addresses[i])
		} else {
			batch = append(batch, addresses[i])
		}
	}

	if err := db.Save(batch); err != nil {
		return nil, nil, fmt.Errorf("DB Saver error: %w", err)
	}
	logger.Info("Успешно сохранил адреса в бд")

	//очистка от пустых значений
	keep := 0
	for i := range twitter {
		if twitter[i] != "" {
			twitter[keep] = twitter[i]
			keep++
		}
	}
	twitter = twitter[:keep]
	return twitter, rem, nil
}
