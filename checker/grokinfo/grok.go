package grokinfo

import (
	"arkham_checker/checker/curlinfo"
	_ "embed"
	"fmt"

	"github.com/go-resty/resty/v2"
)

//go:embed prompt.txt
var prompt string

//go:embed curl.txt
var curl string

type GrokInfo struct {
	Pastebin   *Pastebin
	Curl       *curlinfo.CurlInfo
	GrokSender *GrokSender
}

type GrokSender struct {
	client *resty.Client
	//параметры для чата
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

func (x *GrokInfo) SendMessage(userurl string) (string, error) {
	resp, err := x.grokRequest(userurl)
	if err != nil {
		return "", err
	}
}
func setPrompt(userurl string) string {
	msg := fmt.Sprintf(prompt, userurl)
	return msg
}

func (x *GrokInfo) grokRequest(userurl string) *resty.Response {
	x.GrokSender.Prompt = se
}
