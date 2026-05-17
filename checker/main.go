package main

import (
	"arkham_checker/checker/checker"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		slog.Error("Failed to load .env file")
		os.Exit(1)
	}

	s, err := os.ReadFile("addresses.txt")
	if err != nil {
		slog.Error("Failed to read addreses.txt. Check that file!")
		os.Exit(1)
	}

	addresses := strings.Fields(string(s))
	twitter, failed, stats := collectTwitters(addresses)
	rows := checker.CheckTelegramUsernames(twitter, intEnv("TG_CONCURRENCY", 10))
	twitterOutput(rows)
	writeFailedAddresses(failed)
	slog.Info("Arkham stats", "total", stats.Total, "with_twitter", stats.WithTwitter, "no_twitter", stats.NoTwitter, "failed", stats.Failed)
}

func twitterOutput(rows []checker.TwitterTGRow) {
	f, err := os.Create("result.txt")
	if err != nil {
		slog.Error("Failed to save to result.txt", "msg", err)
		os.Exit(1)
	}

	defer f.Close()

	w := tabwriter.NewWriter(f, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ТВИТТЕР\tТГ")
	for _, row := range rows {
		if strings.TrimSpace(row.Twitter) == "" {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\n", strings.TrimSpace(row.Twitter), strings.TrimSpace(row.Telegram))
	}
	if err := w.Flush(); err != nil {
		slog.Error("Failed to flush output", "err", err)
	}
}

type ArkhamStats struct {
	Total       int
	WithTwitter int
	NoTwitter   int
	Failed      int
}

func collectTwitters(addresses []string) ([]string, []string, ArkhamStats) {
	client := resty.New()
	client.SetTimeout(20 * time.Second)
	maxConns := intEnv("ARKHAM_CONCURRENCY", 20) * 2
	if maxConns < 10 {
		maxConns = 10
	}

	client.SetTransport(&http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   maxConns,
		MaxConnsPerHost:       maxConns,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	})

	var (
		mu      sync.Mutex
		twitter []string
		failed  []string
	)

	sem := make(chan struct{}, intEnv("ARKHAM_CONCURRENCY", 20))
	rps := intEnv("ARKHAM_RPS", 16)

	var limiter <-chan time.Time
	if rps > 0 {
		interval := time.Second / time.Duration(rps)
		if interval <= 0 {
			interval = time.Nanosecond
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		limiter = ticker.C
	}
	var wg sync.WaitGroup
	var statsMu sync.Mutex
	stats := ArkhamStats{Total: len(addresses)}

	for i, adres := range addresses {
		sem <- struct{}{}
		wg.Add(1)

		go func(idx int, adr string) {
			defer wg.Done()
			defer func() { <-sem }()

			if limiter != nil {
				<-limiter
			}
			res := checker.FetchTwitterWithRetryResult(client, adr)

			switch res.Status {
			case checker.FetchOK:
				mu.Lock()
				twitter = append(twitter, res.Twitter)
				mu.Unlock()
				statsMu.Lock()
				stats.WithTwitter++
				statsMu.Unlock()
			case checker.FetchNoTwitter:
				statsMu.Lock()
				stats.NoTwitter++
				statsMu.Unlock()
			default:
				mu.Lock()
				failed = append(failed, strings.TrimSpace(adr))
				mu.Unlock()
				statsMu.Lock()
				stats.Failed++
				statsMu.Unlock()
			}

			twLog := res.Twitter
			if res.Status == checker.FetchNoTwitter {
				twLog = "Nope"
			}
			if res.Status == checker.FetchFailed {
				twLog = "Failed to fetch"
			}
			slog.Info("Fetch wallet", "wallet", adr, "twitter", twLog, "request", idx+1, "status", res.StatusCode)
		}(i, adres)
	}
	wg.Wait()

	return twitter, uniqueNonEmpty(failed), stats
}

func intEnv(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func writeFailedAddresses(addresses []string) {
	if len(addresses) == 0 {
		return
	}
	f, err := os.Create("failed_addresses.txt")
	if err != nil {
		slog.Error("Failed to save failed_addresses.txt", "err", err)
		return
	}
	defer f.Close()

	for _, a := range addresses {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		fmt.Fprintln(f, a)
	}
}

func uniqueNonEmpty(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
