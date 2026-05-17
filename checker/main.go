package main

import (
	"arkham_checker/checker/checker"
	Resolver "arkham_checker/checker/resolver"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"

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
	twitter := checker.CollectTwitters(addresses)
	// tgUsernames := Resolver.CheckUsernames(twitter)
	tgUsernames := Resolver.ExtractUsernames()
	twitterOutput(twitter, tgUsernames)
}

// вывод в .txt ссылок твиттера и юзернеймов тг
func twitterOutput(twitter []string, tgUsernames []string) {
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

	fmt.Fprintf(w, "\n")
	for _, user := range tgUsernames {
		fmt.Fprintf(w, "%s\n", strings.TrimSpace(user))
	}

	if err := w.Flush(); err != nil {
		slog.Error("Failed to flush output", "err", err)
	}
}
