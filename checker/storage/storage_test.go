package storage

import (
	"testing"

	"github.com/dgraph-io/badger/v4"
)

func TestSave(t *testing.T) {
	testaddresses := []string{
		"0x4e814d275567f62145b882773c30b9719d99235a",
		"0x7718f4f65b8b409644c91f86d275086d1902731b",
		"0xc85122bffb1b191934e8eefac73b612a07cc8c7f",
		"0x8ed38e5e74250a8df021671a7e711234a1beb6f0",
		"0xd4173c590f9eaba671a9cfdb280b654574105c72",
		"0x1f6a278ba5a89cb9971cad50c2e5fabe8e7beddc",
		"0xf5b7687c0cfd29637b34ca6f6a53ce843409e8b8",
	}
	//подключение к бд
	opts := badger.DefaultOptions("").WithInMemory(true)
	opts.ValueThreshold = 1024
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatalf("Не удалось открыть базу для теста: %v", err)
	}

	storage := Badger{DB: db}
	defer storage.DB.Close()
	//вызываем наш метод storage.Save
	err = storage.Save(testaddresses)
	if err != nil {
		t.Fatalf("Ошибка сохранения: %v", err)
	}
	t.Log("Успешно сохранил пачку")
	//проверка что значения есть
	if err := db.View(func(txn *badger.Txn) error {
		for _, key := range testaddresses {
			item, err := txn.Get([]byte(key))
			if err != nil {
				t.Fatalf("Ошибка получения ключа %s", key)
			}
			k := item.Key()
			t.Logf("Успешно найден ключ %s", string(k))
		}

		return nil
	}); err != nil {
		t.Fatalf("Ошибка просмотра значений в бд %v", err)
	}
}

func TestUniqueAddresses(t *testing.T) {
	testaddresses := []string{
		"0x4e814d275567f62145b882773c30b9719d99235a",
		"0x7718f4f65b8b409644c91f86d275086d1902731b",
		"0xc85122bffb1b191934e8eefac73b612a07cc8c7f",
		"0x8ed38e5e74250a8df021671a7e711234a1beb6f0",
		"0xd4173c590f9eaba671a9cfdb280b654574105c72",
		"0x1f6a278ba5a89cb9971cad50c2e5fabe8e7beddc",
		"0xf5b7687c0cfd29637b34ca6f6a53ce843409e8b8",
	}
	//подключение к бд
	opts := badger.DefaultOptions("").WithInMemory(true)
	opts.ValueThreshold = 1024
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatalf("Не удалось открыть базу для теста: %v", err)
	}

	storage := Badger{DB: db}
	defer storage.DB.Close()

	//вызываем наш метод storage.Save
	err = storage.Save(testaddresses)
	if err != nil {
		t.Fatalf("Ошибка сохранения: %v", err)
	}
	t.Log("Успешно сохранил пачку для теста")

	testaddresses2 := []string{
		"0x8ed38e5e74250a8df021671a7e711234a1beb6f0",
		"0xd4173c590f9eaba671a9cfdb280b654574105c72",
		"0x1f6a278ba5a89cb9971cad50c2e5fabe8e7beddc",
		"0xf5b7687c0cfd29637b34ca6f6a53ce843409e8b8",
		"1",
		"2",
	}
	//тестируем storage.UniqueAddresses
	addresses, err := storage.UniqueAddresses(testaddresses2)
	if err != nil {
		t.Fatalf("Ошибка проверки пачки: %v", err)
	}
	t.Logf("Успешное тестирование. Уникальные адреса: %s", addresses)
}
