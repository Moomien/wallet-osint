package gsheets

import (
	log "arkham_checker/checker/logger"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	credentialsFile = "gsheets/credentials.json"
	tokenFile       = "gsheets/token.json"
)

type GSheets struct {
	service     *sheets.Service
	wallets     []string
	twitter     []string
	tgUsernames []string
	pastes      []string
	mu          sync.RWMutex
	log         *log.Logger
}

func NewGhsheet(ctx context.Context, twitter []string,
	wallets []string, tgUsernames []string, pastes []string) (*GSheets, error) {

	logger, err := log.NewLogger("GSheets")
	if err != nil {
		return nil, err
	}

	bs, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, err
	}

	config, err := google.ConfigFromJSON(bs, sheets.SpreadsheetsScope)
	if err != nil {
		return nil, fmt.Errorf("Ошибка OAuth-конфига: %w", err)
	}

	client, err := getClient(ctx, config)
	if err != nil {
		return nil, err
	}

	service, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return &GSheets{
		service:     service,
		twitter:     twitter,
		wallets:     wallets,
		tgUsernames: tgUsernames,
		pastes:      pastes,
		log:         logger,
	}, nil
}

func (g *GSheets) Close() error {
	if g.log != nil {
		return g.log.Close()
	}
	return nil
}

// создаёт таблицу, заполняет и возвращает ссылку на неё
func (g *GSheets) CreateTable() (string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	maxLen := max(len(g.wallets), len(g.tgUsernames), len(g.pastes))

	if maxLen == 0 {
		return "", fmt.Errorf("нет данных для создания таблицы")
	}

	values := make([][]any, 0, maxLen+1)
	values = append(values, []any{"Wallet", "Twitter", "TgUsernames", "Paste"})

	for i := 0; i < maxLen; i++ {
		row := []any{
			valueOrEmpty(g.wallets, i),
			valueOrEmpty(g.twitter, i),
			valueOrEmpty(g.tgUsernames, i),
			valueOrEmpty(g.pastes, i),
		}
		values = append(values, row)
	}

	vr := sheets.ValueRange{
		Values: values,
	}

	spreadsheet := &sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: "Wallet checker",
		},
	}

	createdSheet, err := g.service.Spreadsheets.Create(spreadsheet).Do()
	if err != nil {
		return "", fmt.Errorf("не удалось создать таблицу: %w", err)
	}

	_, err = g.service.Spreadsheets.Values.Update(
		createdSheet.SpreadsheetId,
		"Sheet1!A1",
		&vr,
	).ValueInputOption("RAW").Do()
	if err != nil {
		return "", fmt.Errorf("не удалось обновить значения: %w", err)
	}

	g.log.Info("Таблица создана: %s", createdSheet.SpreadsheetUrl)
	return createdSheet.SpreadsheetUrl, nil
}

func valueOrEmpty(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return ""
}

func getClient(ctx context.Context, config *oauth2.Config) (*http.Client, error) {
	token, err := tokenFromFile(tokenFile)
	if err != nil {
		token, err = getTokenFromWeb(ctx, config)
		if err != nil {
			return nil, err
		}
		if err := saveToken(tokenFile, token); err != nil {
			return nil, err
		}
	}

	return config.Client(ctx, token), nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("не получилось прочитать токен из файла: %w", err)
	}
	defer f.Close()
	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)

	return token, err
}

func getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL(
		"state-token",
		oauth2.AccessTypeOffline,
	)
	fmt.Printf("Перейди по ссылке: %s\n", authURL)
	fmt.Print("Введи полученный код: ")

	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ввода: %w", err)
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return nil, fmt.Errorf("код не может быть пустым")
	}

	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить токен: %w", err)
	}

	return token, nil
}

func saveToken(filepath string, token *oauth2.Token) error {
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("не удалось сохранить токен: %w", err)
	}
	defer file.Close()
	json.NewEncoder(file).Encode(token)
	return nil
}
