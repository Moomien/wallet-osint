package Notion

import (
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/joho/godotenv"
)

func randomstring(count int) []string {
	const letters = "abcdefghlkmnopqrst1234567890privetpriveptrdsfksdfksdkfo23j3290dsoajsdkjksj"
	res := make([]string, count)
	for i := range count {
		length := rand.IntN(len(letters)-10) + 10
		str := make([]byte, length)
		for j := range length {
			str[j] = letters[rand.IntN(len(letters))]
		}
		res[i] = string(str)
	}
	return res
}
func TestNotio(t *testing.T) {
	if err := godotenv.Load(); err != nil {
		t.Fatal(err)
	}

	txts := randomstring(10)
	urls := make(chan string, 100)
	client, err := NewNotionClient()
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for _, text := range txts {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			if err := client.CreatePaste(u, urls); err != nil {
				t.Log(err)
			}
		}(text)
	}

	go func() {
		wg.Wait()
		close(urls)
	}()

	result := []string{}
	var mu sync.Mutex
	for url := range urls {
		mu.Lock()
		result = append(result, url)
		mu.Unlock()
	}
	t.Log(result)
}
