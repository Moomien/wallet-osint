# FyneApp - GUI для Wallet Checker

GUI-приложение на Fyne v2 для просмотра результатов работы чекера.

## Структура пакета

```
fyneapp/
├── app.go           # Главная структура App и её методы
├── table.go         # Таблица и её рендеринг
├── toolbar.go       # Панель инструментов (New, Open, Save, Save As)
├── analysis.go      # Окно анализа
├── session.go       # Работа с сессиями
├── builder.go       # BuildRows функция
└── run.go           # Точка входа Run()
```

## Быстрый старт

```go
package main

import "yourproject/fyneapp"

func main() {
    wallets := []string{"0x123...", "0x456..."}
    twitters := []string{"@user1", "@user2"}
    tgUsernames := []string{"tg_user1", "tg_user2"}
    texts := []string{"Analysis text 1", "Analysis text 2"}
    
    rows := fyneapp.BuildRows(wallets, twitters, tgUsernames, texts)
    fyneapp.Run(rows)
}
```

## Основные компоненты

### Row - модель данных

```go
type Row struct {
    Wallet     string  // Адрес кошелька
    Twitter    string  // Twitter username
    TgUsername string  // Telegram username
    PasteTitle string  // Заголовок анализа (обычно "View")
    PasteText  string  // Текст анализа
}
```

### App - главное приложение

```go
app := fyneapp.NewApp()
app.LoadRows(rows)        // Загрузить данные
app.Clear()               // Очистить таблицу
app.Refresh()             // Перерисовать интерфейс
app.ShowAndRun()          // Показать окно и запустить
```

### BuildRows - сборка данных

```go
rows := fyneapp.BuildRows(
    wallets,     // []string
    twitters,    // []string
    tgUsernames, // []string
    texts,       // []string
)
```

## Особенности

### Главное окно
- Размер: 1200x800
- Название: "Wallet Checker"

### Панель инструментов
- **New**: Создать новую сессию
- **Open**: Открыть сохраненную сессию
- **Save**: Сохранить текущую сессию
- **Save As**: Сохранить как новую сессию

### Таблица

Колонки:
1. **Wallet** - кнопка, копирует адрес в буфер обмена
2. **Twitter** - кнопка, копирует username в буфер обмена
3. **Telegram** - кнопка, копирует username в буфер обмена
4. **Analysis** - кнопка "View", открывает окно анализа

### Окно анализа

При нажатии на "View" открывается отдельное окно с:
- Заголовком с адресом кошелька
- Текстом анализа (прокручиваемый, выделяемый, копируемый)

## Работа с сессиями

```go
// Сохранение
session := &fyneapp.Session{
    Name: "my_session",
    Rows: rows,
}
err := fyneapp.SaveSession("sessions/my_session.json", session)

// Загрузка
loadedSession, err := fyneapp.LoadSession("sessions/my_session.json")

// Список сессий
sessions, err := fyneapp.GetSessions()
```

## TODO

Текущая реализация содержит заглушки для:
- Диалогов сохранения/открытия файлов
- Иконок на панели инструментов
- Полной реализации обработчиков кнопок панели

Эти функции можно реализовать по мере необходимости.

## Требования

- Go 1.16+
- Fyne v2
