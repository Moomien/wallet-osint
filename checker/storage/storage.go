package storage

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

type Storage interface {
	Save(addresses []string) error
	UniqueAddresses(addresses []string) (addreses []string, err error)
	Close() error
}

type Badger struct {
	DB *badger.DB
}

func NewBadgerDB() (*Badger, error) {
	opts := badger.DefaultOptions("./addresses_db")
	//настраиваем чтобы база хранила только ключи sst
	opts.ValueThreshold = 1024
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("Ошибка создания или открытия бд")
	}
	return &Badger{DB: db}, nil
}

func (db *Badger) Close() error {
	return db.DB.Close()
}

// проход в цикле по пачке адресов, добавление в батч и сохранение батча
func (b *Badger) Save(addresses []string) error {
	batch := b.DB.NewWriteBatch()

	defer batch.Cancel()

	for _, key := range addresses {
		err := batch.Set([]byte(key), nil)
		if err != nil {
			return fmt.Errorf("Ошибка сохранения в батч: %w", err)
		}
	}

	if err := batch.Flush(); err != nil {
		return err
	}
	return nil
}

// проверить каждую строку в бд
// если есть то убираем из конечного слайса адресов
func (b *Badger) UniqueAddresses(adr []string) (addresses []string, err error) {
	err = b.DB.View(func(txn *badger.Txn) error {
		for _, key := range adr {
			bs := []byte(key)
			_, err := txn.Get(bs)
			//проверяем что адрес новый и добавляем в конечный слайс
			if errors.Is(err, badger.ErrKeyNotFound) {
				addresses = append(addresses, key)
			} else if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("Ошибка проверки и подготовки данных: %w", err)
	}

	return addresses, nil
}
