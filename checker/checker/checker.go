package checker

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
)

func CollectTwitters(addresses []string) []string {
	client := resty.New()
	twitter := make([]string, len(addresses))
	sem := make(chan struct{}, 8) // ~20 rps
	var wg sync.WaitGroup

	for i, adres := range addresses {
		wg.Add(1)
		go func(idx int, adr string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			tw := fetchTwitterWithRetry(client, adr)
			if tw != "Nope" && tw != "Failed to fetch" {
				twitter[idx] = tw
			}
			slog.Info("Fetch wallet", "wallet", adr, "twitter", tw)
		}(i, adres)
	}
	wg.Wait()

	return twitter
}

func fetchTwitterWithRetry(client *resty.Client, address string) string {
	url := arkhamURL(address)

	exponenntialBackoff := []int{1, 2, 4, 8, 16}
	for attempt := 0; attempt <= len(exponenntialBackoff); attempt++ {
		resp, err := newArkhamRequest(client).Get(url)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to request, retrying %d", attempt), "err", err)
			continue
		}

		if resp.StatusCode() == 200 {
			twitter := gjson.Get(resp.String(), "arkhamEntity.twitter")
			if !twitter.Exists() {
				twitter := "Nope"
				return twitter
			}
			return twitter.String()
		}

		slog.Error("Arkham api returned bad status", "status", resp.StatusCode(), "attempt", attempt+1)
	}

	return "Failed to fetch"
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
