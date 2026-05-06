package main

import (
	"arkham_checker/checker"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"text/tabwriter"

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
	twitter := collectTwitters(addresses)
	twitterOutput(twitter)
}

func twitterOutput(twitter []string) {
	res, err := os.Create("result.txt")
	if err != nil {
		slog.Error("Failed to create .txt file")
		os.Exit(1)
	}

	defer res.Close()

	w := tabwriter.NewWriter(res, 0, 0, 3, ' ', 0)
	for _, item := range twitter {
		fmt.Fprintf(w, "%s", strings.TrimSpace(item))
	}

	if err := w.Flush(); err != nil {
		slog.Error("Failed to flush output", "err", err)
	}
}

func collectTwitters(addresses []string) []string {
	client := resty.New()
	twitter := make([]string, len(addresses))
	sem := make(chan struct{}, 10) // ~20 rps
	var wg sync.WaitGroup

	for i, adres := range addresses {
		wg.Add(1)
		go func(idx int, adr string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			tw := checker.FetchTwitterWithRetry(client, adr)
			if tw != "Nope" && tw != "Failed to fetch" {
				twitter[idx] = tw
			}
			slog.Info("Fetch wallet", "wallet", adr, "twitter", tw)
		}(i, adres)
	}
	wg.Wait()

	return twitter
}
