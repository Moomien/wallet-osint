package Resolver

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

// TestResolverIntegration - интеграционный тест для проверки резолвинга реальных юзернеймов
func TestResolverIntegration(t *testing.T) {
	if err := godotenv.Load(); err != nil {
		t.Fatal(err)
	}
	appIDStr := os.Getenv("APP_ID")
	appHash := os.Getenv("APP_HASH")
	botToken := os.Getenv("BOT_TOKEN")

	if appIDStr == "" || appHash == "" || botToken == "" {
		t.Skip("Пропускаем интеграционный тест: требуются APP_ID, APP_HASH, BOT_TOKEN")
	}

	var appID int
	if _, err := fmt.Sscanf(appIDStr, "%d", &appID); err != nil {
		t.Fatalf("Неверный формат APP_ID: %v", err)
	}

	resolver, err := NewResolver(appID, appHash, botToken)
	if err != nil {
		t.Fatalf("Не удалось создать resolver: %v", err)
	}

	testUsernames := []string{
		"durov",
		"telegram",
		"vitalik",
		"ethereum",
		"nonexistent_user_12345678",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	usernames, err := resolver.CheckUsernames(ctx, testUsernames)
	if err != nil {
		t.Fatalf("Ошибка резолвинга: %v", err)
	}

	t.Logf("Найдено %d из %d юзернеймов", len(usernames), len(testUsernames))

	if len(usernames) == 0 {
		t.Error("Не найдено ни одного юзернейма, ожидалось хотя бы 1")
	}

	foundMap := make(map[string]bool)
	for _, u := range usernames {
		foundMap[u] = true
	}

	for _, testUser := range testUsernames {
		if foundMap[testUser] {
			t.Logf("Найден: %s", testUser)
		} else {
			t.Logf("Не найден: %s", testUser)
		}
	}

	if !foundMap["durov"] {
		t.Error("Ожидалось что 'durov' будет найден")
	}
	if !foundMap["telegram"] {
		t.Error("Ожидалось что 'telegram' будет найден")
	}

	if foundMap["nonexistent_user_12345678"] {
		t.Error("Несуществующий пользователь не должен быть найден")
	}
}

// TestResolverEmptyInput - тест с пустым списком
func TestResolverEmptyInput(t *testing.T) {
	appIDStr := os.Getenv("APP_ID")
	appHash := os.Getenv("APP_HASH")
	botToken := os.Getenv("BOT_TOKEN")

	if appIDStr == "" || appHash == "" || botToken == "" {
		t.Skip("Пропускаем тест: требуются APP_ID, APP_HASH, BOT_TOKEN")
	}

	var appID int
	fmt.Sscanf(appIDStr, "%d", &appID)

	resolver, err := NewResolver(appID, appHash, botToken)
	if err != nil {
		t.Fatalf("Не удалось создать resolver: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	usernames, err := resolver.CheckUsernames(ctx, []string{})
	if err != nil {
		t.Fatalf("Ошибка резолвинга: %v", err)
	}

	if len(usernames) != 0 {
		t.Errorf("Ожидалось 0 юзернеймов, получено %d", len(usernames))
	}
}

// TestResolverCreation - тест создания resolver
func TestResolverCreation(t *testing.T) {
	tests := []struct {
		name     string
		appID    int
		appHash  string
		botToken string
		wantErr  bool
	}{
		{
			name:     "Valid credentials",
			appID:    12345,
			appHash:  "test_hash",
			botToken: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
			wantErr:  false,
		},
		{
			name:     "Empty app hash",
			appID:    12345,
			appHash:  "",
			botToken: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
			wantErr:  false, // Resolver создастся, но упадёт при попытке подключиться
		},
		{
			name:     "Zero app ID",
			appID:    0,
			appHash:  "test_hash",
			botToken: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
			wantErr:  false, // Resolver создастся, но упадёт при попытке подключиться
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := NewResolver(tt.appID, tt.appHash, tt.botToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewResolver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && resolver == nil {
				t.Error("NewResolver() вернул nil без ошибки")
			}
			if resolver != nil {
				if resolver.client == nil {
					t.Error("Resolver.client is nil")
				}
				if resolver.api == nil {
					t.Error("Resolver.api is nil")
				}
				if resolver.logger == nil {
					t.Error("Resolver.logger is nil")
				}
			}
		})
	}
}
