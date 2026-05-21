package grokinfo

import (
	"os"

	"github.com/go-resty/resty/v2"
)

//здесь будет реализация сохранения текста на pastebin

type Pastebin struct {
	Client *resty.Client
	//
	//то что идет в боди для каждого запроса
	ApiKey string
	Option string
}

func (p *Pastebin) setOption(option string) {
	p.Option = option
}

func NewPastebin() *Pastebin {
	client := resty.New()
	return &Pastebin{
		Client: client,
		ApiKey: os.Getenv("pastebinApikey"),
	}
}

// делает пост запрос и возвращает полученную ссылку
func (p *Pastebin) CreatePaste(text, option string) (string, error) {
	p.setOption(option)

	req := p.pastebinrequest(text)
	resp, err := req.Post("https://pastebin.com/api/api_post.php")
	if err != nil {
		return "", err
	}

	body := resp.Body()
	return string(body), nil
}

func (p *Pastebin) pastebinrequest(text string) *resty.Request {
	request := p.Client.NewRequest().SetFormData(map[string]string{
		"api_dev_key":    p.ApiKey,
		"api_paste_code": text,
		"api_option":     p.Option,
	})
	return request
}
