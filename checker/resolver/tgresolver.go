package Resolver

import (
	"context"
	"fmt"
	"log/slog"
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
						slog.Error(fmt.Sprintf("RPC : %s for %s", rpcErr.Message, user))
					}
					//ретраим если попали в лимит
					if tgerr.Is(err, "FLOOD_WAIT") {
						for i := 0; i < 6; i++ {
							fmt.Println("Попали в лимит, ретрай:", i)
							retry := retryafter(i)
							time.Sleep(retry)

							resolved, err = resolver.api.ContactsResolveUsername(ctx,
								&tg.ContactsResolveUsernameRequest{Username: user})

							if tgerr.Is(err, "FLOOD_WAIT") {
								continue
							}

							if len(resolved.Chats) > 0 || len(resolved.Users) > 0 {
								mu.Lock()
								usernames = append(usernames, user)
								mu.Unlock()
							}
						}
					}

					return
				}

				//если все норм никаких лимитов - добавляем
				if len(resolved.Users) > 0 || len(resolved.Chats) > 0 {
					mu.Lock()
					usernames = append(usernames, user)
					mu.Unlock()
				} else {
					return
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

func NewResolver(appID int, appHash, botToken string) *Resolver {
	client := telegram.NewClient(appID, appHash, telegram.Options{})
	api := client.API()

	return &Resolver{
		client:   client,
		api:      api,
		botToken: botToken,
	}
}

func retryafter(i int) time.Duration {
	backoff := []time.Duration{
		time.Millisecond * 100, 200 * time.Millisecond,
		400 * time.Millisecond, 800 * time.Millisecond,
		1600 * time.Millisecond, 5 * time.Second,
	}

	return backoff[i]
}
