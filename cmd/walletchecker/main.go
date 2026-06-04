package main

import (
	"arkham_checker/internal/checker"
	"arkham_checker/internal/fyneapp"
	"arkham_checker/internal/grokclient"
	"arkham_checker/internal/interceptor"
	log "arkham_checker/internal/logger"
	Resolver "arkham_checker/internal/resolver"
	"arkham_checker/internal/session"
	"arkham_checker/internal/storage"
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	// Загружаем .env
	if err := godotenv.Load(); err != nil {
		fmt.Println("Предупреждение: .env файл не найден")
	}

	// Получаем cookie из .env
	cookie := os.Getenv("cookie")
	if cookie == "" {
		fmt.Println("Ошибка: переменная 'cookie' не установлена в .env файле")
		os.Exit(1)
	}

	// Проверяем аргументы командной строки для proxy
	// Использование: go run main.go proxy
	proxyFlag := ""
	if len(os.Args) > 1 && os.Args[1] == "proxy" {
		proxyFlag = "proxy"
		fmt.Println("Режим с прокси активирован (используется proxy.txt)")
	}

	// Создаем контекст с поддержкой отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Обработка Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nПолучен сигнал прерывания, завершаем работу...")
		cancel()
	}()

	// Инициализируем компоненты
	logger, err := log.NewLogger("main")
	if err != nil {
		fmt.Printf("Ошибка создания логгера: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	db, err := storage.NewBadgerDB()
	if err != nil {
		logger.Error("Ошибка создания БД", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Читаем адреса из файла
	addresses, err := readAddresses("addresses.txt")
	if err != nil {
		logger.Error("Ошибка чтения адресов", "error", err)
		os.Exit(1)
	}

	if len(addresses) == 0 {
		logger.Info("Нет адресов для обработки")
		return
	}

	// Фильтруем уникальные адреса
	uniqueAddresses, err := db.UniqueAddresses(addresses)
	if err != nil {
		logger.Error("Ошибка фильтрации адресов", "error", err)
		os.Exit(1)
	}

	logger.Info(fmt.Sprintf("Всего адресов: %d, уникальных: %d", len(addresses), len(uniqueAddresses)))

	// Настраиваем Arkham клиент
	arkhamCfg := checker.ArkhamConfig{
		Cookie:    cookie,
		ProxyFile: "proxy.txt",
		RPS:       10,
		Burst:     2,
		Flag:      proxyFlag,
	}

	arkham, err := checker.NewArkhamClient(arkhamCfg)
	if err != nil {
		logger.Error("Ошибка создания Arkham клиента", "error", err)
		os.Exit(1)
	}

	// Собираем Twitter аккаунты
	logger.Info("Начинаем сбор Twitter аккаунтов...")
	twitters, remaining, err := checker.CollectTwitters(ctx, db, arkham, uniqueAddresses)
	if err != nil {
		logger.Error("Ошибка сбора Twitter", "error", err)
		os.Exit(1)
	}

	if len(remaining) > 0 {
		logger.Warn(fmt.Sprintf("Не обработано адресов: %d", len(remaining)))
	}

	logger.Info(fmt.Sprintf("Собрано Twitter аккаунтов: %d", len(twitters)))

	if len(twitters) == 0 {
		logger.Info("Нет Twitter аккаунтов для дальнейшей обработки")
		return
	}

	// Резолвим Telegram юзернеймы
	logger.Info("Начинаем резолвинг Telegram...")
	tgUsernames := make([]string, len(twitters))

	// Очищаем урлы
	cleanTwitters := make([]string, len(twitters))
	for i, tw := range twitters {
		username := strings.TrimPrefix(tw, "https://twitter.com/")
		username = strings.TrimPrefix(username, "https://x.com/")
		cleanTwitters[i] = username
	}

	// Получаем параметры для Telegram resolver из .env
	appIDStr := os.Getenv("APP_ID")
	appHash := os.Getenv("APP_HASH")
	botToken := os.Getenv("BOT_TOKEN")

	if appIDStr != "" && appHash != "" && botToken != "" {
		var appID int
		fmt.Sscanf(appIDStr, "%d", &appID)

		tgResolver := Resolver.NewResolver(appID, appHash, botToken)

		resolvedUsernames, err := tgResolver.CheckUsernames(ctx, cleanTwitters)
		if err != nil {
			logger.Warn("Ошибка резолвинга Telegram", "error", err)
		} else {
			// Сопоставляем результаты с исходными твиттерами
			resolvedMap := make(map[string]bool)
			for _, username := range resolvedUsernames {
				resolvedMap[username] = true
			}

			for i, cleanTw := range cleanTwitters {
				if resolvedMap[cleanTw] {
					tgUsernames[i] = cleanTw
					logger.Info(fmt.Sprintf("Резолвнут: %s -> %s", twitters[i], cleanTw))
				}
			}
		}
	} else {
		logger.Warn("Telegram резолвинг пропущен (не настроены APP_ID, APP_HASH, BOT_TOKEN в .env)")
	}

	// Инициализируем Grok клиент
	logger.Info("Инициализация Grok...")
	cacheSession, err := session.NewCache(true, "grok.com")
	if err != nil {
		logger.Error("Ошибка создания session cache", "error", err)
		os.Exit(1)
	}
	defer cacheSession.Close()

	inter, err := interceptor.NewInterceptor(cacheSession)
	if err != nil {
		logger.Error("Ошибка создания interceptor", "error", err)
		os.Exit(1)
	}

	grokConfig, err := grokclient.NewGrokConfig(inter)
	if err != nil {
		logger.Error("Ошибка создания Grok config", "error", err)
		os.Exit(1)
	}
	defer grokConfig.Close()

	// Читаем промпт
	promptBytes, err := os.ReadFile("prompt.txt")
	if err != nil {
		logger.Error("Ошибка чтения prompt.txt", "error", err)
		os.Exit(1)
	}
	prompt := string(promptBytes)

	// Запускаем воркеров для анализа через Grok
	logger.Info("Запуск Grok анализа...")
	pool := grokclient.NewSessionPool(grokConfig)

	grokResults := make([]string, len(twitters))
	resultChan := make(chan struct {
		index  int
		result string
		err    error
	}, len(twitters))

	//чтобы сохранить порядок
	twitterIndexMap := make(map[string]int, len(cleanTwitters))
	for i, username := range cleanTwitters {
		twitterIndexMap[username] = i
	}

	// Обработчик результатов
	var processedCount int
	onResult := func(username, result string, err error) {
		// username уже чистый (без URL префиксов)
		if index, found := twitterIndexMap[username]; found {
			resultChan <- struct {
				index  int
				result string
				err    error
			}{index, result, err}
		}
	}

	// Запускаем воркеры
	go func() {
		err := pool.RunWorkers(ctx, cleanTwitters, onResult, prompt, "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
		if err != nil {
			logger.Error("Ошибка выполнения воркеров", "error", err)
		}
		close(resultChan)
	}()

	// Собираем результаты
	for res := range resultChan {
		processedCount++
		if res.err != nil {
			logger.Error(fmt.Sprintf("[%d/%d] Ошибка анализа %s", processedCount, len(twitters), twitters[res.index]), "error", res.err)
			grokResults[res.index] = ""
		} else {
			logger.Info(fmt.Sprintf("[%d/%d] Анализ завершен: %s", processedCount, len(twitters), twitters[res.index]))
			grokResults[res.index] = res.result
		}
	}

	logger.Info("Анализ завершен, запуск GUI...")

	// Готовим данные для GUI
	rows := fyneapp.BuildRows(uniqueAddresses, twitters, tgUsernames, grokResults)

	// Запускаем GUI
	fyneapp.Run(rows)

	logger.Info("Программа завершена")
}

// readAddresses читает адреса из файла
func readAddresses(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var addresses []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			addresses = append(addresses, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return addresses, nil
}
