package main

import (
	"arkham_checker/checker/checker"
	"arkham_checker/checker/grokclient"
	paste "arkham_checker/checker/grokclient/paste_service"
	"arkham_checker/checker/gsheets"
	"arkham_checker/checker/interceptor"
	Resolver "arkham_checker/checker/resolver"
	"arkham_checker/checker/session"
	"arkham_checker/checker/storage"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/joho/godotenv"
)

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
		fmt.Printf("Удалено %d строк. Уникальные адреса: %d\n",
			len(addresses)-len(uniqaddresses), len(uniqaddresses))
	}

	//collect
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	//go run main.go proxy
	proxyflag := ""
	if len(os.Args) > 1 {
		proxyflag = os.Args[1]
	}
	
	arkham, err := checker.NewArkhamClient(checker.ArkhamConfig{
		Cookie:    os.Getenv("cookie"),
		ProxyFile: "proxy.txt",
		RPS:       15,
		Burst:     5,
		Flag:      proxyflag,
	})
	if err != nil {
		slog.Error("Failed to initialize Arkham client", "err", err)
		os.Exit(1)
	}

	twitter, remaining, err := checker.CollectTwitters(ctx, db, arkham, uniqaddresses)
	if err != nil {
		slog.Error("CollectTwitters failed", "err", err)
		os.Exit(1)
	}

	//удаляем отчеканные адреса и оставляем остатки, если есть
	slog.Info("Saving remaining to addresses.txt!")
	str := strings.Join(remaining, "\n")
	bs := []byte(str)
	if err := os.WriteFile("addresses.txt", bs, 0644); err != nil {
		slog.Error("Failed to save remaining to addresses.txt", "err", err)
	} else {
		slog.Info("File addresses.txt successfully updated", "Remaining", len(remaining))
	}

	if len(twitter) == 0 {
		slog.Info("Было найдено 0 ссылок твиттер, поэтому ничего не выведу :(")
		return
	}

	slog.Info("Найдено Twitter аккаунтов", "count", len(twitter))
	
	appID, err := strconv.Atoi(os.Getenv("APP_ID"))
	if err != nil {
		slog.Error("failed to convert string(app_id) to int(app_id)", "err", err)
		os.Exit(1)
	}
	appHash := os.Getenv("APP_HASH")
	botToken := os.Getenv("BOT_TOKEN")
	
	resolver := Resolver.NewResolver(appID, appHash, botToken)
	tgUsernames, err := resolver.CheckUsernames(ctx, twitter)
	if err != nil {
		slog.Error("Failed to check telegram usernames", "err", err)
		os.Exit(1)
	}
	slog.Info("Резолвинг Telegram завершен", "count", len(tgUsernames))

	// Сохраняем в result.txt (старый вывод)
	if err := twitterOutput(twitter, tgUsernames); err != nil {
		slog.Error("Failed to output twitter", "err", err)
	} else {
		slog.Info("Успешно сохранил твиттер и тг в result.txt")
	}

	// Отправка на Grok через пул воркеров
	slog.Info("Запускаю обработку через Grok...")
	grokResults, pastes := processWithGrok(ctx, twitter)
	slog.Info("Обработка Grok завершена", "успешно", len(grokResults))

	// Сохранение в Google Sheets
	slog.Info("Создаю Google Sheets таблицу...")
	if err := saveToGoogleSheets(ctx, uniqaddresses, twitter, tgUsernames, pastes); err != nil {
		slog.Error("Не удалось создать Google Sheets таблицу", "err", err)
	} else {
		slog.Info("Данные успешно сохранены в Google Sheets")
	}

	slog.Info("Работа завершена успешно!")
}

// processWithGrok обрабатывает Twitter юзернеймы через Grok
func processWithGrok(ctx context.Context, twitterUsers []string) (map[string]string, []string) {
	promptBytes, err := os.ReadFile("grokclient/prompt.txt")
	if err != nil {
		slog.Error("Не удалось прочитать prompt.txt", "err", err)
		return nil, nil
	}
	prompt := string(promptBytes)

	useragent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	cacheSession, err := session.NewCache(true, "grok.com")
	if err != nil {
		slog.Error("Не удалось создать session cache", "err", err)
		return nil, nil
	}
	defer cacheSession.Close()

	inter, err := interceptor.NewInterceptor(cacheSession)
	if err != nil {
		slog.Error("Не удалось создать interceptor", "err", err)
		return nil, nil
	}
	defer inter.Close()

	grokConfig, err := grokclient.NewGrokConfig("grokclient/sessions", inter)
	if err != nil {
		slog.Error("Не удалось создать GrokConfig", "err", err)
		return nil, nil
	}
	defer grokConfig.Close()

	notion, err := paste.NewNotionClient()
	if err != nil {
		slog.Error("Не удалось создать GitHub Gist клиент", "err", err)
		return nil, nil
	}
	defer notion.Close()

	pool := grokclient.NewSessionPool(grokConfig)

	var mu sync.Mutex
	results := make(map[string]string)
	pastes := make([]string, len(twitterUsers))
	userIndexMap := make(map[string]int)

	// Создаем мапу для сохранения порядка юзернеймов
	for i, user := range twitterUsers {
		userIndexMap[user] = i
	}

	gistURLs := make(chan string, len(twitterUsers))

	onResult := func(username, result string, err error) {
		mu.Lock()
		userIndex := userIndexMap[username]
		mu.Unlock()

		if err != nil {
			slog.Error("Ошибка обработки пользователя", "username", username, "err", err)
			mu.Lock()
			results[username] = ""
			pastes[userIndex] = ""
			mu.Unlock()
			return
		}

		mu.Lock()
		results[username] = result
		mu.Unlock()

		slog.Info("Обработан пользователь", "username", username)

		// Сохраняем результат в GitHub Gist
		go func(user, text string, index int) {
			err := notion.CreatePaste(text, gistURLs)
			if err != nil {
				slog.Error("Не удалось создать Gist", "username", user, "err", err)
				mu.Lock()
				pastes[index] = ""
				mu.Unlock()
				return
			}

			// Получаем URL из канала
			select {
			case url := <-gistURLs:
				mu.Lock()
				pastes[index] = url
				mu.Unlock()
				slog.Info("Создан Gist", "username", user, "url", url)
			case <-time.After(10 * time.Second):
				slog.Error("Таймаут получения Gist URL", "username", user)
				mu.Lock()
				pastes[index] = ""
				mu.Unlock()
			}
		}(username, result, userIndex)
	}

	if err := pool.RunWorkers(ctx, twitterUsers, onResult, prompt, useragent); err != nil {
		slog.Error("Ошибка при запуске воркеров", "err", err)
		return nil, nil
	}

	// Ждем завершения создания всех Gist
	time.Sleep(5 * time.Second)

	return results, pastes
}

// saveToGoogleSheets сохраняет данные в Google Sheets
func saveToGoogleSheets(ctx context.Context, wallets, twitter, tgUsernames, pastes []string) error {
	gs, err := gsheets.NewGhsheet(ctx)
	if err != nil {
		return fmt.Errorf("создание GSheets клиента: %w", err)
	}
	defer gs.Close()

	url, err := gs.CreateTable(gsheets.TableData{
		Wallets:     wallets,
		Twitter:     twitter,
		TgUsernames: tgUsernames,
		Pastes:      pastes,
	})
	if err != nil {
		return fmt.Errorf("создание таблицы: %w", err)
	}

	slog.Info("Таблица создана", "url", url)
	fmt.Printf("\nGoogle Sheets таблица создана: %s\n\n", url)

	return nil
}

// вывод в .txt ссылок твиттера и юзернеймов тг
func twitterOutput(twitter []string, tgUsernames []string) error {
	res, err := os.OpenFile("result.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to read or create .txt file: %w", err)
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
		return fmt.Errorf("failed to flush output: %w", err)
	}
	return nil
}
