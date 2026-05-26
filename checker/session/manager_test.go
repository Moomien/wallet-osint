package session

import (
	"sync"
	"testing"
)

func TestManager(t *testing.T) {
	sessManager, err := NewCache()
	if err != nil {
		t.Fatal(err)
	}
	defer sessManager.Close()
	defer sessManager.Logger.Close()
	var wg sync.WaitGroup
	for range len(sessManager.data) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sessManager.CheckSession()
		}()
	}
	wg.Wait()
	t.Log("Горутины успешно выполнены!")
	cookie, err := sessManager.GetSession()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(cookie)
}
