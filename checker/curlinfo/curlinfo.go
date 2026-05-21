package curlinfo

import (
	"strings"

	"github.com/mattn/go-shellwords"
)

type CurlInfo struct {
	URL     string
	Headers map[string]string
	Body    string
	Cookie  string
}

func NewCurlInfo(curl string) (*CurlInfo, error) {
	curl = strings.ReplaceAll(curl, `\`, "")
	args, err := shellwords.Parse(curl)
	if err != nil {
		return nil, err
	}

	info := &CurlInfo{
		URL:     args[1],
		Headers: make(map[string]string),
	}

	for i := range args {
		switch args[i] {
		case "curl":
			continue
		case "-H", "--headers":
			if i+1 < len(args) {
				parts := strings.SplitN(args[i+1], ":", 2)
				if len(parts) == 2 {
					info.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
				i++
			}
		case "-b", "--cookie":
			if i+1 < len(args) {
				info.Cookie = args[i+1]
				i++
			}
		case "--data-raw":
			if i+1 < len(args) {
				info.Body = args[i+1]
			}
		default:
			continue
		}
	}
	return info, nil
}
