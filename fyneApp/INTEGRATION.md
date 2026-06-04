# Интеграция FyneApp с существующим проектом

Это руководство по интеграции GUI в ваш проект WalletChecker.

## Быстрый старт

### 1. Добавьте GUI в существующий код

В вашем `checker/main.go` или любом другом месте:

```go
package main

import (
    "arkham_checker/fyneapp"
    // ... остальные импорты
)

func main() {
    // Ваша существующая логика чекера
    wallets := []string{}
    twitters := []string{}
    tgUsernames := []string{}
    texts := []string{}
    
    // ... код сбора данных ...
    
    // После сбора всех данных запускаем GUI
    rows := fyneapp.BuildRows(wallets, twitters, tgUsernames, texts)
    fyneapp.Run(rows)
}
```

### 2. Запуск демо

Демо-приложение находится в `fyneapp/demo/main.go`:

```bash
# Компиляция
go build -o demo.exe ./fyneapp/demo

# Запуск
./demo.exe
```

## Архитектура интеграции

### Вариант 1: CLI флаг для GUI

```go
package main

import (
    "flag"
    "arkham_checker/fyneapp"
)

func main() {
    showGUI := flag.Bool("gui", false, "Show GUI after checking")
    flag.Parse()
    
    // Ваша логика чекера
    wallets, twitters, tgUsernames, texts := runChecker()
    
    if *showGUI {
        rows := fyneapp.BuildRows(wallets, twitters, tgUsernames, texts)
        fyneapp.Run(rows)
    } else {
        // Вывод результатов в консоль или файл
        printResults(wallets, twitters, tgUsernames, texts)
    }
}
```

### Вариант 2: Отдельная команда

Создайте отдельный исполняемый файл для GUI:

```go
// cmd/gui/main.go
package main

import (
    "arkham_checker/fyneapp"
    "encoding/json"
    "os"
)

func main() {
    // Загружаем данные из файла результатов
    data := loadResultsFromFile("result.json")
    
    rows := fyneapp.BuildRows(
        data.Wallets,
        data.Twitters,
        data.TgUsernames,
        data.Texts,
    )
    
    fyneapp.Run(rows)
}
```

### Вариант 3: Встроенный режим

Запускайте чекер и GUI одновременно:

```go
package main

import (
    "arkham_checker/fyneapp"
)

func main() {
    app := fyneapp.NewApp()
    
    go func() {
        // Запускаем чекер в фоне
        for {
            wallets, twitters, tgUsernames, texts := checkNextBatch()
            
            rows := fyneapp.BuildRows(wallets, twitters, tgUsernames, texts)
            app.LoadRows(rows)
            app.Refresh()
        }
    }()
    
    app.ShowAndRun()
}
```

## Работа с существующими данными

### Загрузка из файла

```go
package main

import (
    "arkham_checker/fyneapp"
    "bufio"
    "os"
    "strings"
)

func loadFromFile(filename string) []string {
    file, err := os.Open(filename)
    if err != nil {
        return []string{}
    }
    defer file.Close()
    
    var lines []string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
    }
    
    return lines
}

func main() {
    wallets := loadFromFile("wallets.txt")
    twitters := loadFromFile("twitters.txt")
    tgUsernames := loadFromFile("telegram.txt")
    texts := loadFromFile("analysis.txt")
    
    rows := fyneapp.BuildRows(wallets, twitters, tgUsernames, texts)
    fyneapp.Run(rows)
}
```

### Интеграция с вашей базой данных

Если вы используете BadgerDB (как видно в проекте):

```go
package main

import (
    "arkham_checker/fyneapp"
    "github.com/dgraph-io/badger/v4"
)

func loadFromBadger(db *badger.DB) ([]string, []string, []string, []string) {
    var wallets, twitters, tgUsernames, texts []string
    
    db.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        it := txn.NewIterator(opts)
        defer it.Close()
        
        for it.Rewind(); it.Valid(); it.Next() {
            item := it.Item()
            key := item.Key()
            
            item.Value(func(val []byte) error {
                // Парсинг ваших данных
                // wallets = append(wallets, ...)
                return nil
            })
        }
        
        return nil
    })
    
    return wallets, twitters, tgUsernames, texts
}

func main() {
    db, err := badger.Open(badger.DefaultOptions("./addresses_db"))
    if err != nil {
        panic(err)
    }
    defer db.Close()
    
    wallets, twitters, tgUsernames, texts := loadFromBadger(db)
    
    rows := fyneapp.BuildRows(wallets, twitters, tgUsernames, texts)
    fyneapp.Run(rows)
}
```

## API Reference

### BuildRows

```go
func BuildRows(
    wallets []string,
    twitters []string,
    tgUsernames []string,
    texts []string,
) []Row
```

Собирает строки из отдельных массивов. Автоматически выравнивает длины массивов.

### Run

```go
func Run(rows []Row)
```

Простейший способ запуска GUI. Создает приложение, загружает данные и показывает окно.

### App Methods

```go
app := fyneapp.NewApp()           // Создать новое приложение
app.LoadRows(rows)                // Загрузить данные
app.Clear()                       // Очистить таблицу
app.Refresh()                     // Обновить интерфейс
app.ShowAndRun()                  // Показать окно и запустить
```

## Примеры использования

### Минимальный пример

```go
rows := fyneapp.BuildRows(
    []string{"0x123..."},
    []string{"@user"},
    []string{"tg_user"},
    []string{"Analysis text"},
)
fyneapp.Run(rows)
```

### С контролем приложения

```go
app := fyneapp.NewApp()
app.LoadRows(rows)

// Можно выполнить дополнительные настройки
// app.Window().SetFullScreen(true)

app.ShowAndRun()
```

### Динамическое обновление

```go
app := fyneapp.NewApp()

go func() {
    for data := range resultChannel {
        rows := fyneapp.BuildRows(data.Wallets, data.Twitters, data.TgUsernames, data.Texts)
        app.LoadRows(rows)
        app.Refresh()
    }
}()

app.ShowAndRun()
```

## Сборка для Windows

```bash
# Обычная сборка
go build -o walletchecker-gui.exe ./cmd/gui

# Сборка без консольного окна (только GUI)
go build -ldflags -H=windowsgui -o walletchecker-gui.exe ./cmd/gui
```

## Требования

- Go 1.16+
- Fyne v2 (уже установлен в проекте)
- Windows: работает из коробки
- Linux: требует установки X11 dev libraries
- macOS: работает из коробки

## Примечания

1. **Не изменяет существующий код**: Пакет полностью независим
2. **Работает с готовыми данными**: GUI не знает о деталях чекера
3. **Простая интеграция**: Один вызов функции для запуска
4. **Расширяемость**: Легко добавить новые функции в будущем
