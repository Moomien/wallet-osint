package Resolver

import (
	log "arkham_checker/internal/logger"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

type Resolver struct {
	client   *telegram.Client
	api      *tg.Client
	botToken string
	logger   log.Log
}

func (resolver *Resolver) CheckUsernames(ctx context.Context, addresses []string) (usernames []string, err error) {
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	err = resolver.client.Run(ctx, func(ctx context.Context) error {
		if _, err := resolver.client.Auth().Bot(ctx, resolver.botToken); err != nil {
			return err
		}

		wg.Add(len(addresses))
		sem := make(chan struct{}, 5)

		for _, user := range addresses {
			go func(user string) {
				defer wg.Done()

				sem <- struct{}{}
				defer func() { <-sem }()

				resolved, err := resolver.api.
					ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: user})

				if err != nil {
					if rpcErr, ok := tgerr.As(err); ok {
						resolver.logger.Error("RPC error", "message", rpcErr.Message, "user", user)
					}

					// Ретраим только если попали в FLOOD_WAIT
					if tgerr.Is(err, "FLOOD_WAIT") {
						for i := 0; i < 6; i++ {
							resolver.logger.Info("Попали в лимит, ретрай", "attempt", i, "user", user)
							retry := retryafter(i)
							time.Sleep(retry)

							resolved, err = resolver.api.ContactsResolveUsername(ctx,
								&tg.ContactsResolveUsernameRequest{Username: user})

							// Если снова FLOOD_WAIT - продолжаем ретрай
							if err != nil && tgerr.Is(err, "FLOOD_WAIT") {
								continue
							}

							// Если успешно или другая ошибка - выходим из цикла
							break
						}
					}

					// Если после всех ретраев всё ещё ошибка - выходим
					if err != nil {
						return
					}
				}

				// Если все норм - добавляем юзернейм
				if resolved != nil && (len(resolved.Users) > 0 || len(resolved.Chats) > 0) {
					mu.Lock()
					usernames = append(usernames, user)
					mu.Unlock()
				}
			}(user)
		}
		wg.Wait()

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to run telegram client: %w", err)
	}

	return usernames, nil
}

func NewResolver(appID int, appHash, botToken string) (*Resolver, error) {
	logger, err := log.NewLogger("resolver")
	if err != nil {
		return nil, fmt.Errorf("создание логгера resolver: %w", err)
	}

	client := telegram.NewClient(appID, appHash, telegram.Options{})
	api := client.API()

	return &Resolver{
		client:   client,
		api:      api,
		botToken: botToken,
		logger:   logger,
	}, nil
}

func retryafter(i int) time.Duration {
	backoff := []time.Duration{
		time.Millisecond * 100, 200 * time.Millisecond,
		400 * time.Millisecond, 800 * time.Millisecond,
		1600 * time.Millisecond, 5 * time.Second,
	}

	return backoff[i]
}
