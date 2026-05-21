package curlinfo

import (
	_ "embed"
	"testing"
)

//go:embed test.txt
var testTemplate string

func TestCurlInfo(t *testing.T) {
	info, err := NewCurlInfo(testTemplate)
	if err != nil {
		t.Logf("Ошибка: %v", err)
	}

	t.Logf("%+v\n", info.URL)
	t.Logf("%+v\n\n\n", info.Headers)
	t.Logf("%+v\n", info.Body)
	t.Logf("%+v\n", info.Cookie)
}
