package grokclient

import (
	"arkham_checker/checker/interceptor"
	log "arkham_checker/checker/logger"
	paste "arkham_checker/checker/paste_service"
	_ "embed"
	"fmt"
	"regexp"

	"github.com/go-resty/resty/v2"
)

//go:embed prompt.txt
var prompt string

//go:embed curl.txt
var curl string

type GrokClient struct {
	Paste  paste.Paste
	client *resty.Client
	prompt string
	log    *log.Logger
}

func NewGrokClient() (*GrokClient, error) {
	p := paste.NewPastebin()
	c := resty.New()

	logger, err := log.NewLogger("grokclient")
	if err != nil {
		return nil, fmt.Errorf("создание логгера grokclient: %w", err)
	}

	gc := &GrokClient{
		Paste:  p,
		client: c,
		prompt: prompt,
		log:    logger,
	}

	return gc, nil
}

// func (x *GrokClient) SendMessage(ctx context.Context, request *interceptor.CapturedRequest, userurl string) (string, error) {
// 	resp, err :=
// }

func (x *GrokClient) grokRequest(request *interceptor.CapturedRequest, userurl string) *resty.Request {
	msg := request.Body
	re := regexp.MustCompile(`("message"\s*:\s*")[^"]*(")`)
	res := re.ReplaceAllString(msg, `${1}`+x.setPrompt(userurl)+`${2}`)
	return x.client.NewRequest().SetHeaders(request.Headers).SetBody(res)
}

// берет юзернейм и подсовывает его в промпт
func (x *GrokClient) setPrompt(userurl string) string {
	msg := fmt.Sprintf(x.prompt, userurl)
	return msg
}
