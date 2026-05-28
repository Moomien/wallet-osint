package paste

import (
	"errors"
	"os"

	"github.com/go-resty/resty/v2"
)

//реализация сохранения текста на сервис pastebin

type Paste interface {
	CreatePaste(text string) (string, error)
}

type Pastebin struct {
	Client *resty.Client
	ApiKey string
	Option string
}

func NewPastebin() (*Pastebin, error) {
	client := resty.New()
	env := os.Getenv("pastebinApikey")
	if env == "" {
		return nil, errors.New(".env не считалось, что то случилось.")
	}
	return &Pastebin{
		Client: client,
		ApiKey: os.Getenv("pastebinApikey"),
	}, nil
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
	p.Option = option
}

func (p *Pastebin) pastebinrequest(text string) *resty.Request {
	request := p.Client.NewRequest().SetFormData(map[string]string{
		"api_dev_key":    p.ApiKey,
		"api_paste_code": text,
		"api_option":     p.Option,
	})
	return request
}
