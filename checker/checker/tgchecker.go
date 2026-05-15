package checker

import (
	"context"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

type TwitterTGRow struct {
	Twitter  string
	Telegram string
}

func CheckTelegramUsernames(twitters []string, concurrency int) []TwitterTGRow {
	if concurrency <= 0 {
		concurrency = 10
	}

	rows := make([]TwitterTGRow, 0, len(twitters))
	handles := make([]string, 0, len(twitters))
	for _, tw := range twitters {
		tw = strings.TrimSpace(tw)
		if tw == "" {
			continue
		}
		twitterURL, handle := normalizeTwitter(tw)
		rows = append(rows, TwitterTGRow{Twitter: twitterURL})
		handles = append(handles, handle)
	}

	appID, appHash, botToken, ok := telegramBotConfig()
	if !ok {
		return rows
	}

	client := telegram.NewClient(appID, appHash, telegram.Options{})
	_ = client.Run(context.Background(), func(ctx context.Context) error {
		if _, err := client.Auth().Bot(ctx, botToken); err != nil {
			return err
		}

		api := client.API()

		sem := make(chan struct{}, concurrency)
		var wg sync.WaitGroup

		for i, handle := range handles {
			if handle == "" {
				continue
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, username string) {
				defer wg.Done()
				defer func() { <-sem }()

				exists, err := telegramUsernameExists(ctx, api, username)
				if err != nil || !exists {
					return
				}
				rows[idx].Telegram = "@" + username
			}(i, handle)
		}

		wg.Wait()
		return nil
	})
	return rows
}

func telegramUsernameExists(ctx context.Context, api *tg.Client, username string) (bool, error) {
	_, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: username,
	})
	if err == nil {
		return true, nil
	}

	if rpcErr, ok := tgerr.As(err); ok {
		if rpcErr.IsOneOf("USERNAME_NOT_OCCUPIED", "USERNAME_INVALID") {
			return false, nil
		}
	}

	return false, err
}

func telegramBotConfig() (appID int, appHash string, botToken string, ok bool) {
	idStr := firstEnv("TG_APP_ID", "TG_API_ID", "TELEGRAM_APP_ID", "TELEGRAM_API_ID")
	appHash = firstEnv("TG_APP_HASH", "TG_API_HASH", "TELEGRAM_APP_HASH", "TELEGRAM_API_HASH")
	botToken = firstEnv("TG_BOT_TOKEN", "TELEGRAM_BOT_TOKEN", "BOT_TOKEN")
	if idStr == "" || appHash == "" || botToken == "" {
		return 0, "", "", false
	}

	id, err := strconv.Atoi(strings.TrimSpace(idStr))
	if err != nil {
		return 0, "", "", false
	}

	return id, strings.TrimSpace(appHash), strings.TrimSpace(botToken), true
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func normalizeTwitter(in string) (twitterURL string, handle string) {
	in = strings.TrimSpace(in)
	if in == "" {
		return "", ""
	}

	if strings.HasPrefix(in, "@") {
		h := strings.TrimPrefix(in, "@")
		h = strings.Trim(h, "/")
		h = strings.TrimSpace(h)
		return "https://twitter.com/" + h, h
	}

	if strings.Contains(in, "twitter.com/") || strings.Contains(in, "x.com/") {
		u, err := url.Parse(in)
		if err == nil {
			p := strings.Trim(u.Path, "/")
			if p != "" {
				seg := strings.Split(p, "/")[0]
				seg = path.Clean("/" + seg)
				seg = strings.Trim(seg, "/")
				if seg != "" {
					return "https://twitter.com/" + seg, seg
				}
			}
		}
	}

	h := strings.Trim(in, "/")
	if strings.Contains(h, "?") {
		h = strings.SplitN(h, "?", 2)[0]
	}
	if strings.Contains(h, "#") {
		h = strings.SplitN(h, "#", 2)[0]
	}
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	h = strings.TrimPrefix(h, "twitter.com/")
	h = strings.TrimPrefix(h, "x.com/")
	h = strings.TrimPrefix(h, "www.twitter.com/")
	h = strings.TrimPrefix(h, "www.x.com/")
	h = strings.Trim(h, "/")

	if h == "" {
		return in, ""
	}

	return "https://twitter.com/" + h, h
}
