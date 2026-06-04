package Notion

import (
	log "arkham_checker/checker/logger"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var (
	ErrBadRequest = errors.New("invalid json")                            // 400
	ErrAuth       = errors.New("auth error, something with bearer token") // 401, 403
	ErrRateLimit  = errors.New("ratelimited")                             // 429
	ErrServer     = errors.New("notion server error")                     // 501, 503
)

type Notion struct {
	client *resty.Client
	log    log.Log
}

func NewNotionClient() (*Notion, error) {
	log, err := log.NewLogger("Notion_Service")
	if err != nil {
		return nil, err
	}

	bearer := os.Getenv("Notion")
	c := resty.New()
	headers := map[string]string{
		"Authorization":  "Bearer " + bearer,
		"Notion-Version": "2022-06-28",
		"Content-Type":   "application/json",
	}

	return &Notion{
		client: c.SetHeaders(headers),
		log:    log,
	}, nil
}

func (n *Notion) Close() error {
	if n.log != nil {
		return n.log.Close()
	}
	return nil
}

func newNotionError(code int, message string) error {
	errMap := map[int]error{
		400: ErrBadRequest,
		401: ErrAuth,
		403: ErrAuth,
		429: ErrRateLimit,
		501: ErrServer,
		503: ErrServer,
	}

	if errMap[code] == nil {
		return fmt.Errorf("notion неизвестная ошибка [%d]: %s", code, message)
	}
	return fmt.Errorf("notion [%d]: %s", code, message)
}

func (n *Notion) CreatePaste(text string, urls chan<- string) error {
	backoff := []time.Duration{
		time.Millisecond * 100,
		time.Millisecond * 200,
		time.Millisecond * 400,
		time.Millisecond * 800,
		time.Millisecond * 1600,
	}
	body, err := buildBody(text)
	if err != nil {
		return err
	}

	resp := &resty.Response{}
	for i := range backoff {
		jitter := rand.IntN(500)

		resp, err = n.client.NewRequest().SetBody(body).Post("https://api.notion.com/v1/pages")
		if err != nil {
			return fmt.Errorf("ошибка отправления запроса в Notion: %w", err)
		}
		if resp.StatusCode() == 400 {
			n.log.Info("Notion 400 response: %s", resp.String())
			return fmt.Errorf("notion: %w", newNotionError(resp.StatusCode(), resp.String()))
		}

		if resp.StatusCode() == 429 {
			n.log.Info("Notion: словили 429, ретраим...")
			time.Sleep(backoff[i] + time.Duration(jitter)*time.Millisecond)
			continue
		}

		if resp.StatusCode() == 401 || resp.StatusCode() == 403 {
			n.log.Info("Notion 401 or 403 response: %s", resp.String())
			return fmt.Errorf("notion: %w", newNotionError(resp.StatusCode(), resp.String()))
		}

		if resp.StatusCode() == 501 || resp.StatusCode() == 503 {
			n.log.Info("Notion 501 or 503 response: %s", resp.String())
			return fmt.Errorf("notion: %w", newNotionError(resp.StatusCode(), resp.String()))
		}

		if resp.StatusCode() == 200 {
			n.log.Info("Notion успех!")
			urls <- gjson.Get(resp.String(), "url").String()
			return nil
		}
	}
	return fmt.Errorf("%w", newNotionError(resp.StatusCode(), resp.String()))
}

func buildBody(text string) (string, error) {
	json := `{
    "parent": { "page_id": "374a4a5d0928809b8825f281e5e29e7b" },
    "properties": {
        "title": {
            "title": [{ "text": { "content": "Паста" } }]
        }
    },
    "children": [
        {
            "object": "block",
            "type": "paragraph",
            "paragraph": {
                "rich_text": [{ "text": { "content": "твой текст здесь" } }]
            }
        }
    ]
}`

	if body, err := sjson.Set(json, "children.0.paragraph.rich_text.0.text.content", text); err == nil {
		return body, nil
	}
	return "", errors.New("не получилось создать body для запроса в Notion")
}
