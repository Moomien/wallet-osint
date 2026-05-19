package main

import (
	"arkham_checker/checker/checker"
	Resolver "arkham_checker/checker/resolver"
	"arkham_checker/checker/storage"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
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
	db, err := storage.NewBadgerDB()
	if err != nil {
		slog.Error("New DB", "Error", err)
		os.Exit(1)
	}
	uniqaddresses, err := db.UniqueAddresses(addresses)
	if err != nil {
		slog.Error("DB unique", "Error", err)
		os.Exit(1)
	}
	if len(uniqaddresses) != len(addresses) {
		fmt.Printf("Удалено %d строк.\n Уникальные адреса: %d",
			len(addresses)-len(uniqaddresses), len(uniqaddresses))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	twitter, remaining := checker.CollectTwitters(ctx, db, uniqaddresses)
	if len(remaining) > 0 {
		str := strings.Join(remaining, "\n")
		bs := []byte(str)
		if err := os.WriteFile("addresses.txt", bs, 0644); err != nil {
			slog.Error("Failed to save remaining to addresses.txt", "err", err)
		}
	}
	tgUsernames := Resolver.CheckUsernames(twitter)

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

	//удаляем мусорные строки из слайса
	twitter = slices.DeleteFunc(twitter, func(s string) bool {
		return s == ""
	})

	w := tabwriter.NewWriter(res, 0, 0, 3, ' ', 0)
	tglen := len(tgUsernames)
	fmt.Fprintf(w, "TWITTER \t TELEGRAM\n")
	for i, item := range twitter {
		if i < tglen {
			fmt.Fprintf(w, "%s \t %s\n", item, tgUsernames[i])
		}
		fmt.Fprintf(w, "%s", item)
	}

	if err := w.Flush(); err != nil {
		slog.Error("Failed to flush output", "err", err)
	}
}
