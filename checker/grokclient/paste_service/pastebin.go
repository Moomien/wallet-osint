package paste

import (
	log "arkham_checker/checker/logger"
	"errors"
	"fmt"
	"os"

	"github.com/go-resty/resty/v2"
)

//реализация сохранения текста на сервис pastebin

type Paste interface {
	CreatePaste(text string) (string, error)
}

type Pastebin struct {
	client *resty.Client
	apiKey string
	option string
	log    *log.Logger
}

func NewPastebin() (*Pastebin, error) {
	logger, err := log.NewLogger("pastebin")
	if err != nil {
		return nil, fmt.Errorf("создание логгера: %w", err)
	}

	client := resty.New()
	env := os.Getenv("pastebinApikey")
	if env == "" {
		return nil, errors.New(".env не считалось, что то случилось.")
	}
	return &Pastebin{
		client: client,
		apiKey: os.Getenv("pastebinApikey"),
		log:    logger,
	}, nil
}

func (p *Pastebin) Close() error {
	return p.log.Close()
}

// делает пост запрос и возвращает полученную ссылку
func (p *Pastebin) CreatePaste(text string) (string, error) {
	p.setOption("paste")

	req := p.pastebinrequest(text)
	resp, err := req.Post("https://pastebin.com/api/api_post.php")
	if err != nil {
		return "", err
	}

	body := resp.Body()
	return string(body), nil
}

func (p *Pastebin) setOption(option string) {
	p.option = option
}

func (p *Pastebin) pastebinrequest(text string) *resty.Request {
	request := p.client.NewRequest().SetFormData(map[string]string{
		"api_dev_key":    p.apiKey,
		"api_paste_code": text,
		"api_option":     p.option,
	})
	return request
}

// для тестов
func (p *Pastebin) getApiKey() string {
	return p.apiKey
}
