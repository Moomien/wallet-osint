package gsheets

import (
	"context"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

func generateRandomStrings(count int) []string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	result := make([]string, count)
	for i := range count {
		length := rand.Intn(10) + 5
		s := make([]byte, length)
		for j := range length {
			s[j] = letters[rand.Intn(len(letters))]
		}
		result[i] = string(s)
	}
	return result
}

func TestGsheets(t *testing.T) {
	slice1 := generateRandomStrings(3)
	slice2 := generateRandomStrings(7)
	slice3 := generateRandomStrings(5)
	slice4 := generateRandomStrings(10)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	gsheet, err := NewGhsheet(ctx, slice1, slice2, slice3, slice4)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer gsheet.Close()

	url, err := gsheet.CreateTable()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(url)
}
