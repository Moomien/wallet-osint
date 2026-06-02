package main

import (
	"arkham_checker/checker/checker"
	Resolver "arkham_checker/checker/resolver"
	"arkham_checker/checker/storage"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/joho/godotenv"
)

// я хз как еще можно расширить приложение.
// может как-то сделать анализ челиков с помощью grok.
// сделать чтобы сохраняло инфу по челикам. сохраняло линк на твиттер, юз тг, ссылка pastebin с инфой о них.
func main() {
	//настройка вывода логов в терминал и отдельный файл
	file, err := os.OpenFile("app.log", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	multiWriter := io.MultiWriter(os.Stdout, file)
	logger := slog.New(slog.NewJSONHandler(multiWriter, nil))
	slog.SetDefault(logger)
	slog.Info("Application started", "time", time.Now())

	err = godotenv.Load()
	if err != nil {
		slog.Error("Failed to load .env file")
		os.Exit(1)
	}

	s, err := os.ReadFile("addresses.txt")
	if err != nil {
		slog.Error("Failed to read addreses.txt. Check that file!")
		os.Exit(1)
	}

	//storage
	addresses := strings.Fields(string(s))
	db, err := storage.NewBadgerDB()
	if err != nil {
		slog.Error("New DB", "Error", err)
		os.Exit(1)
	}
	defer db.Close()

	//отсев использованных ранее строк
	uniqaddresses, err := db.UniqueAddresses(addresses)
	if err != nil {
		slog.Error("DB unique", "Error", err)
		os.Exit(1)
	}

	if len(uniqaddresses) != len(addresses) {
		fmt.Printf("Удалено %d строк. Уникальные адреса: %d",
			len(addresses)-len(uniqaddresses), len(uniqaddresses))
	}

	//collect
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	//go run main.go proxy
	proxyflag := os.Args[1]
	twitter, remaining := checker.CollectTwitters(ctx, db, uniqaddresses, proxyflag)

	//удаляем отчеканные адреса и оставляем остатки, если есть
	slog.Info("Saving remaining to addresses.txt!")
	str := strings.Join(remaining, "\n")
	bs := []byte(str)
	if err := os.WriteFile("addresses.txt", bs, 0644); err != nil {
		slog.Error("Failed to save remaining to addresses.txt", "err", err)
	} else {
		slog.Info("File addresses.txt successfully updated", "Remaining", len(remaining))
	}

	//резолвинг юзернеймов в тг
	if len(twitter) != 0 {
		tgUsernames := Resolver.CheckUsernames(twitter)
		twitterOutput(twitter, tgUsernames)
		slog.Info("Успешно сохранил твиттер и тг в result.txt")
		return
	}
	slog.Info("Было найдено 0 ссылок твиттер, поэтому ничего не выведу :(")
}

// вывод в .txt ссылок твиттера и юзернеймов тг
func twitterOutput(twitter []string, tgUsernames []string) {
	res, err := os.OpenFile("result.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("Failed to read or create .txt file")
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
		tg := "-"
		if i < tglen {
			tg = tgUsernames[i]
		}
		fmt.Fprintf(w, "%s \t %s\n", item, tg)
	}

	if err := w.Flush(); err != nil {
		slog.Error("Failed to flush output", "err", err)
	}
}
