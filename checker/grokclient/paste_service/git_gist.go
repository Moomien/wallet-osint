package paste

import (
	"fmt"

	log "arkham_checker/checker/logger"
	"math/rand/v2"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// сервис паст с использованием гитхаба
type GitGist struct {
	client *resty.Client
	token  string
	log    *log.Logger
}

func NewGitGist() (*GitGist, error) {
	logger, err := log.NewLogger("gitgist")
	if err != nil {
		fmt.Println("не получи")
		return nil, err
	}
	token := os.Getenv("gist_token")
	headers := map[string]string{
		"Accept":               "application/vnd.github+json",
		"Authorization":        "Bearer " + token,
		"X-GitHub-Api-Version": "2026-03-10",
	}
	client := resty.New().SetHeaders(headers)

	return &GitGist{
		client: client,
		token:  token,
		log:    logger,
	}, nil
}

func (gist *GitGist) Close() error {
	if gist.log != nil {
		return gist.log.Close()
	}
	return nil
}

// отправка реквеста на GitGist и сразу парсинг
func (gist *GitGist) CreatePaste(text string, urls chan<- string) (string, error) {
	jitter := rand.IntN(3)
	backoff := []time.Duration{time.Millisecond * 100, time.Millisecond * 200, time.Millisecond * 400, time.Millisecond * 800}
	for i := range backoff {
		b := `{"private":false,"files":{"@username":{"content":"privet"}}}`
		body, err := sjson.Set(b, "files.@username.content", text)
		if err != nil {
			return "", fmt.Errorf("создание тела реквеста: %w", err)
		}

		resp, err := gist.client.NewRequest().SetBody(body).Post("https://api.github.com/gists")
		if err != nil {
			return "", fmt.Errorf("не получилось создать пасту:%w", err)
		}

		if resp.StatusCode() == 401 {
			gist.log.Info("проверь токен гитхаба в .env")
			return "", fmt.Errorf("неверные credentials для GitGist")
		}

		if resp.StatusCode() == 429 {
			gist.log.Info(fmt.Sprintf("retry-after %d", backoff[i]+time.Duration(jitter)))
			time.Sleep(backoff[i] + time.Duration(jitter))
			continue
		}

		if resp.StatusCode() == 201 {
			gist.log.Info("успешно создана паста")
			urls <- gjson.Get(resp.String(), "files.@username.raw_url").String()
			return "", nil
		}
	}
	return "", fmt.Errorf("не получилось создать новую пасту для текста: \n%s\n", text)
}
