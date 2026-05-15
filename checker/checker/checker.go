package checker

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
)

type FetchStatus int

const (
	FetchOK FetchStatus = iota
	FetchNoTwitter
	FetchFailed
)

type FetchResult struct {
	Twitter     string
	Status      FetchStatus
	StatusCode  int
	LastError   error
	RawResponse string
}

func FetchTwitterWithRetryResult(client *resty.Client, address string) FetchResult {
	url := arkhamURL(address)

	backoff := []time.Duration{400 * time.Millisecond, 800 * time.Millisecond, 1500 * time.Millisecond, 3 * time.Second, 6 * time.Second}
	attempts := len(backoff) + 1

	var lastErr error
	var lastStatus int
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := newArkhamRequest(client).Get(url)
		if err != nil {
			lastErr = err
			slog.Error("Failed to request", "attempt", attempt+1, "err", err)
			if attempt < len(backoff) {
				time.Sleep(backoff[attempt])
			}
			continue
		}

		lastStatus = resp.StatusCode()

		if resp.StatusCode() == 200 {
			tw := gjson.Get(resp.String(), "arkhamEntity.twitter")
			if !tw.Exists() {
				return FetchResult{Status: FetchNoTwitter, StatusCode: resp.StatusCode(), RawResponse: resp.String()}
			}
			return FetchResult{Twitter: tw.String(), Status: FetchOK, StatusCode: resp.StatusCode(), RawResponse: resp.String()}
		}

		if resp.StatusCode() == 429 {
			wait := retryAfter(resp)
			slog.Error("Arkham api rate limited", "status", resp.StatusCode(), "sleep", wait, "attempt", attempt+1)
			time.Sleep(wait)
			continue
		}

		slog.Error("Arkham api returned bad status", "status", resp.StatusCode(), "attempt", attempt+1)
		if attempt < len(backoff) {
			time.Sleep(backoff[attempt])
		}
	}

	return FetchResult{Status: FetchFailed, StatusCode: lastStatus, LastError: lastErr}
}

func FetchTwitterWithRetry(client *resty.Client, address string) string {
	res := FetchTwitterWithRetryResult(client, address)
	switch res.Status {
	case FetchOK:
		return res.Twitter
	case FetchNoTwitter:
		return "Nope"
	default:
		return "Failed to fetch"
	}
}

func newArkhamRequest(client *resty.Client) *resty.Request {
	cookie := os.Getenv("cookie")
	return client.NewRequest().SetHeader("Connection", "keep-alive").SetHeader("Accept", "application/json").
		SetHeader("Accept-language", "en-US,en;q=0.9").SetHeader("Origin", "https://intel.arkm.com").
		SetHeader("pragma", "no-cache").SetHeader("priority", "u=1, i").SetHeader("Referer", "https://intel.arkm.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36").
		SetHeader("Cookie", cookie)
}

func arkhamURL(address string) string {
	return "https://api.arkm.com/intelligence/address/" + strings.TrimSpace(address)
}

func retryAfter(resp *resty.Response) time.Duration {
	h := strings.TrimSpace(resp.Header().Get("Retry-After"))
	if h == "" {
		return time.Minute
	}
	if sec, err := strconv.Atoi(h); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second
	}
	return time.Minute
}
