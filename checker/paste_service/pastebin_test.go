package paste

import (
	"testing"

	"github.com/joho/godotenv"
)

func TestCreatePaste(t *testing.T) {
	if err := godotenv.Load(); err != nil {
		t.Logf("Failed to load .env file: %v", err)
	}
	pastebin := NewPastebin()
	t.Logf("api_dev_key вот такой: %v", pastebin.ApiKey)
	sometxt := "sometxttxtxtxtxttxtxt"
	str, err := pastebin.CreatePaste(sometxt)
	if err != nil {
		t.Errorf("Failed to send post-request: %v", err)
	}
	t.Logf("Ссылка: %s", str)
}
