package grokinfo

import (
	"arkham_checker/checker/curlinfo"
	"context"
	_ "embed"
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
)

//go:embed prompt.txt
var prompt string

//go:embed curl.txt
var curl string

type GrokInfo struct {
	Pastebin   *Pastebin //возможно удалю хз посмотрим
	Curl       *curlinfo.CurlInfo
	GrokSender *GrokSender
}

type GrokSender struct {
	client *resty.Client
	Prompt string
}

func NewGrokInfo() (*GrokInfo, error) {
	info, err := curlinfo.NewCurlInfo(curl)
	if err != nil {
		return nil, err
	}
	return &GrokInfo{
		Pastebin: NewPastebin(),
		Curl:     info,
		GrokSender: &GrokSender{
			client: resty.New(),
		},
	}, nil
}

func (x *GrokInfo) SendMessage(ctx context.Context, userurl string) (string, error) {
	resp, err := x.grokRequest(userurl).SetContext(ctx).Post(x.Curl.URL)
	if err != nil {
		return "", err
	}

	if resp.StatusCode() == 200 {
		body := gjson.Parse(resp.String())
		body.Get()
	} 
	if 
}
func setPrompt(userurl string) string {
	msg := fmt.Sprintf(prompt, userurl)
	return msg
}

func (x *GrokInfo) grokRequest(userurl string) *resty.Request {
	
}
