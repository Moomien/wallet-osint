package Resolver

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
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

func CheckUsernames(addresses []string) (addr []string) {
	resolver := newResolver()

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	ctx := context.Background()

	err := resolver.client.Run(ctx, func(ctx context.Context) error {
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
					ContactsResolveUsername(ctx,
						&tg.ContactsResolveUsernameRequest{Username: user})

				if err != nil {
					if rpcErr, ok := tgerr.As(err); ok {
						fmt.Println("RPC error message:", rpcErr.Message)
						fmt.Println("Username:", user)
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
								addr = append(addr, user)
								mu.Unlock()
							}
						}
					}

					return
				}

				//если все норм никаких лимитов - добавляем
				if len(resolved.Users) > 0 || len(resolved.Chats) > 0 {
					mu.Lock()
					addr = append(addr, user)
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
		slog.Error("failed to run client: ", "error", err)
		os.Exit(1)
	}

	return addr
}

func newResolver() *Resolver {
	appID, err := strconv.Atoi(os.Getenv("APP_ID"))
	if err != nil {
		slog.Error("failed to convert string(app_id) to int(app_id)")
		os.Exit(1)
	}

	appHASH := os.Getenv("APP_HASH")
	client := telegram.NewClient(appID, appHASH, telegram.Options{})
	api := client.API()

	return &Resolver{
		client:   client,
		api:      api,
		botToken: os.Getenv("BOT_TOKEN"),
	}
}

func retryafter(i int) time.Duration {
	backoff := []time.Duration{
		time.Millisecond * 100, 200 * time.Microsecond,
		400 * time.Microsecond, 800 * time.Microsecond,
		1600 * time.Microsecond, 5 * time.Second,
	}

	return backoff[i]
}

// этот метод не будет использован в конечном чекере
// юзы будут сразу доставаться из памяти
// это нужно для теста
// func ExtractUsernames() []string {
// 	// достаем ссылки вида
// 	// https://twitter.com/Username
// 	// https://x.com/Usenrname
// 	twt_urls, err := os.ReadFile("usernames.txt")
// 	if err != nil {
// 		slog.Error("failed load txt")
// 		os.Exit(1)
// 	}

// 	usernames := string(twt_urls)
// 	cleanedTxt := strings.ReplaceAll(usernames, "https://twitter.com/", "")
// 	cleanedTxt = strings.ReplaceAll(cleanedTxt, "https://x.com/", "")

// 	return strings.Split(cleanedTxt, "\r\n")
// }
