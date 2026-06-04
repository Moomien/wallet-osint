package checker

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
)

// ArkhamClient — HTTP-клиент для Arkham Intelligence API
type ArkhamClient struct {
	client    *resty.Client
	cookie    string
	proxyFunc func() string
	flag      string
}

type ArkhamConfig struct {
	Cookie    string
	ProxyFile string // путь к proxy.txt, пустой = без прокси
	RPS       int
	Burst     int
	Flag      string // flag "proxy" or ""
}

func NewArkhamClient(cfg ArkhamConfig) (*ArkhamClient, error) {
	client := resty.New()
	
	if cfg.RPS > 0 && cfg.Burst > 0 {
		limiter := newRestyLimiter(cfg.RPS, cfg.Burst)
		client.SetRateLimiter(limiter)
	}

	var proxyFunc func() string
	if cfg.ProxyFile != "" && cfg.Flag == "proxy" {
		pf, err := getProxyFunc(cfg.ProxyFile)
		if err != nil {
			return nil, fmt.Errorf("ошибка загрузки прокси: %w", err)
		}
		proxyFunc = pf
	}

	return &ArkhamClient{
		client:    client,
		cookie:    cfg.Cookie,
		proxyFunc: proxyFunc,
		flag:      cfg.Flag,
	}, nil
}

// FetchTwitter — получает Twitter username по адресу кошелька.
// Возвращает пустую строку, если Twitter не найден.
func (a *ArkhamClient) FetchTwitter(ctx context.Context, address string) (string, error) {
	url := arkhamURL(address)

	exponentialBackoff := []time.Duration{
		1 * time.Second, 2 * time.Second, 4 * time.Second,
		8 * time.Second, 16 * time.Second, 30 * time.Second,
	}

	for attempt := 0; attempt < 6; attempt++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			req := a.newRequest().SetContext(ctx)
			resp, err := req.Get(url)
			if err != nil {
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				slog.Error(fmt.Sprintf("Failed to request %s, retrying %d", address, attempt+1), "Error", err)
				continue
			}

			if resp.StatusCode() == 429 {
				duration := resp.Header().Get("Retry-After")
				var sleepDur time.Duration
				// джиттер чтобы избежать эффект грохочущего стада
				jitter := time.Duration(rand.Intn(30000)) * time.Millisecond

				dur, err := strconv.ParseInt(duration, 10, 64)
				if err == nil {
					sleepDur = time.Duration(dur)*time.Second + jitter
				}
				if sleepDur == 0 {
					sleepDur = exponentialBackoff[attempt] + jitter
				}
				slog.Info(fmt.Sprintf("429: too many requests, sleep for %v seconds for wallet: %s", sleepDur, address))

				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(sleepDur):
					continue
				}
			}

			if resp.StatusCode() == 200 {
				twitter := gjson.Get(resp.String(), "arkhamEntity.twitter")
				if !twitter.Exists() {
					return "", nil
				}
				return twitter.String(), nil
			}
			
			// Если другой статус код, но не 429
			slog.Warn(fmt.Sprintf("Неожиданный статус код %d для %s", resp.StatusCode(), address))
		}
	}

	return "", fmt.Errorf("failed to fetch after %d attempts", 6)
}

func (a *ArkhamClient) newRequest() *resty.Request {
	req := a.client.NewRequest().
		SetHeader("Connection", "keep-alive").
		SetHeader("Accept", "application/json").
		SetHeader("Accept-language", "en-US,en;q=0.9").
		SetHeader("Origin", "https://intel.arkm.com").
		SetHeader("pragma", "no-cache").
		SetHeader("priority", "u=1, i").
		SetHeader("Referer", "https://intel.arkm.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36").
		SetHeader("Cookie", a.cookie)

	if a.proxyFunc != nil {
		a.client.SetProxy(a.proxyFunc())
	}

	return req
}

func getProxyFunc(proxyFile string) (func() string, error) {
	file, err := os.ReadFile(proxyFile)
	if err != nil {
		return nil, err
	}
	str := strings.Split(string(file), "\r\n")
	if len(str) == 0 || (len(str) == 1 && str[0] == "") {
		return nil, fmt.Errorf("empty proxy file")
	}
	lastProxy := str[rand.Intn(len(str))]

	return func() string {
		newProxy := str[rand.Intn(len(str))]
		for newProxy == lastProxy && len(str) > 1 {
			newProxy = str[rand.Intn(len(str))]
		}
		lastProxy = newProxy
		return newProxy
	}, nil
}

func arkhamURL(address string) string {
	return "https://api.arkm.com/intelligence/address/" + strings.TrimSpace(address)
}
