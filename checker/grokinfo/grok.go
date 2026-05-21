package grokinfo

import "github.com/go-resty/resty/v2"

type GrokInfo struct {
	Pastebin *Pastebin
	Grok     *Grok
}

type Grok struct {
	client *resty.Client
	//параметры для чата
	Prompt string
}

func NewGrokInfo() *GrokInfo {
	return &GrokInfo{
		Pastebin: NewPastebin(),
		Grok: &Grok{
			client: resty.New(),
			Prompt: setprompt(),
		},
	}
}

func setprompt() string {
	text := ""
	return text
}
