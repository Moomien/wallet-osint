package checker

import (
	"arkham_checker/checker/storage"
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
)

// возвращает линки и остатки адресов если есть
func CollectTwitters(ctx context.Context, db *storage.Badger, addresses []string, flag string) ([]string, []string) {
	defer db.DB.Close()

	goLimiter := NewRateLimiter(20, 5)
	client := resty.New().SetRateLimiter(goLimiter)

	twitter := make([]string, len(addresses))
	completed := make([]bool, len(addresses))

	sem := make(chan struct{}, 15) // специально делаем семафор большим чтобы хватило рейтлимитеру
	var wg sync.WaitGroup

	for i, adres := range addresses {
		select {
		case <-ctx.Done():
			goto wait
		default:
			wg.Add(1)
			go func(idx int, adr string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				tw := fetchTwitterWithRetry(ctx, client, adr, flag)
				if tw == "Canceled" {
					return
				}

				if tw != "Nope" && tw != "Failed to fetch" {
					twitter[idx] = tw
					slog.Info(fmt.Sprintf("%d Fetch wallet %s Twitter %s", idx, adr, tw))
					return
				}
				completed[idx] = true
				slog.Info(fmt.Sprintf("%d Failed to fetch wallet %s", idx, adr), "MSG", tw)
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
		slog.Error("DB Saver", "error", err)
		os.Exit(1)
	}
	slog.Info("Успешно сохранил адреса в бд")
	//очистка от пустых значений
	keep := 0
	for i := range twitter {
		if twitter[i] != "" {
			twitter[keep] = twitter[i]
			keep++
		}
	}
	twitter = twitter[:keep]
	return twitter, rem
}

type ctxKey string

const proxykey ctxKey = "request_proxy"

func fetchTwitterWithRetry(ctx context.Context, client *resty.Client, address string, flag string) string {
	url := arkhamURL(address)

	exponenntialBackoff := []time.Duration{
		1 * time.Second, 2 * time.Second, 4 * time.Second,
		8 * time.Second, 16 * time.Second, 30 * time.Second,
	}

	for attempt := 0; attempt < 6; attempt++ {
		select {
		case <-ctx.Done():
			return "Canceled"
		default:
			resp, err := newArkhamRequest(client, flag).SetContext(ctx).Get(url)
			if err != nil {
				if ctx.Err() != nil {
					return "Canceled"
				}
				slog.Error(fmt.Sprintf("Failed to request %s, retrying %d", address, attempt+1), "Error", err)
				continue
			}

			if resp.StatusCode() == 429 {
				duration := resp.Header().Get("Retry-After")
				var sleepDur time.Duration
				//джиттер чтобы избежать эффект грохочущего стада
				jitter := time.Duration(rand.Intn(30000)) * time.Millisecond

				dur, err := strconv.ParseInt(duration, 10, 64)
				if err == nil {
					sleepDur = time.Duration(dur)*time.Second + jitter
				}
				if sleepDur == 0 {
					sleepDur = exponenntialBackoff[attempt] + jitter
				}
				slog.Info(fmt.Sprintf("429: too many requests, sleep for %v seconds for wallet: %s", sleepDur, address))

				select {
				case <-ctx.Done():
					return "Canceled"
				case <-time.After(sleepDur):
					time.Sleep(sleepDur)
					continue
				}
			}

			if resp.StatusCode() == 200 {
				twitter := gjson.Get(resp.String(), "arkhamEntity.twitter")
				if !twitter.Exists() {
					return "Nope"
				}
				return twitter.String()
			}
		}
	}

	return "Failed to fetch"
}

func getProxy() (func() string, error) {
	file, err := os.ReadFile("proxy.txt")
	if err != nil {
		return nil, err
	}
	str := strings.Split(string(file), "\r\n")
	lastProxy := str[rand.Intn(len(str))]

	return func() string {
		newProxy := str[rand.Intn(len(str))]
		for newProxy == lastProxy {
			newProxy = str[rand.Intn(len(str))]
		}
		lastProxy = newProxy
		return newProxy
	}, nil
}

func newArkhamRequest(client *resty.Client, flag string) *resty.Request {
	cookie := os.Getenv("cookie")
	if flag == "proxy" {
		selectproxy, err := getProxy()
		if err != nil {
			return nil
		}
		return client.SetProxy(selectproxy()).NewRequest().SetHeader("Connection", "keep-alive").SetHeader("Accept", "application/json").
			SetHeader("Accept-language", "en-US,en;q=0.9").SetHeader("Origin", "https://intel.arkm.com").
			SetHeader("pragma", "no-cache").SetHeader("priority", "u=1, i").SetHeader("Referer", "https://intel.arkm.com/").
			SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36").
			SetHeader("Cookie", cookie)
	}

	return client.NewRequest().SetHeader("Connection", "keep-alive").SetHeader("Accept", "application/json").
		SetHeader("Accept-language", "en-US,en;q=0.9").SetHeader("Origin", "https://intel.arkm.com").
		SetHeader("pragma", "no-cache").SetHeader("priority", "u=1, i").SetHeader("Referer", "https://intel.arkm.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36").
		SetHeader("Cookie", cookie)
}

func arkhamURL(address string) string {
	return "https://api.arkm.com/intelligence/address/" + strings.TrimSpace(address)
}
