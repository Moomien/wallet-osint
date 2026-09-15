## Wallet Doxxer

Пробиваешь крипто кошельки, вытаскиваешь твиттеры, телеграм ники, прогоняешь через Grok AI и отображаешь всё в GUI таблице. Всё автоматом, без лишних движений.

### Что умеет

- Читает кошельки из `addresses.txt`
- Проверяет уникальность в BadgerDB (не прогоняет одно и то же дважды)
- Параллельно пробивает по Arkham API и тянет привязанные Twitter аккаунты
- Резолвит твиттер ники в телеграм юзернеймы (через Telegram API)
- Прогоняет каждый твиттер аккаунт через Grok AI (анализ профиля, активность, связи)
- Отображает результаты в GUI приложении (Fyne)
- Позволяет сохранять и загружать сессии работы

### Структура данных

Проект использует структуру папки `data/` для хранения всех рабочих данных:

```
data/
├── app-sessions/        # Сохраненные сессии GUI
├── db/                  # BadgerDB база данных (уникальные адреса)
├── logs/                # Логи приложения
│   └── app.log
└── grok/
    ├── grok.com/
    │   ├── cache.json  # Кэш сессий Grok
    │   └── cookies/    # Cookie файлы для Grok.com
    └── xai/
        └── accounts/   # Аккаунты для Grok API (.txt файлы)
```

### Как запустить

1. Настрой `.env`:

```env
cookie=...твой cookie из intel.arkm.com...
APP_ID=7006882
APP_HASH=0bb67c4ed967fb83c78fd19fce0b3ea4
BOT_TOKEN=8612017277:AAGTqgIxQvdt19a0uI2lyfOKDaFfFlDkJ6s
```

2. Подготовь файлы:
   - `addresses.txt` — список кошельков (по одному на строку)
   - `proxy.txt` (опционально) — прокси в формате `http://логин:пароль@адрес:порт`
   - `data/grok/grok.com/cookies/` — JSON файлы с **куками** для Grok
   - `data/grok/xai/accounts/` — .txt файлы с аккаунтами xai
    Для работы с xAI необходимо предоставить curl запрос к апи XAI. Его можно взять из devtools. 
   - `prompt.txt` — промпт для анализа через Grok

3. Запусти:

```bash
# Запуск без прокси
go run cmd/walletchecker/main.go

# Запуск с прокси (использует proxy.txt)
go run cmd/walletchecker/main.go proxy

# Или скомпилируй и запусти
go build -o walletchecker.exe ./cmd/walletchecker
walletchecker.exe          # без прокси
walletchecker.exe proxy    # с прокси
```

**Примечание:** Браузер всегда запускается в headless режиме (без видимого окна).

### Что внутри

- `internal/checker/` — модуль сбора твиттеров через Arkham
- `internal/resolver/` — резолвинг твиттер → телеграм
- `internal/grokclient/` — пул воркеров для Grok AI
- `internal/fyneapp/` — GUI приложение
- `internal/storage/` — BadgerDB для отсева дубликатов
- `internal/session/` — кеш и управление браузерными сессиями
- `internal/interceptor/` — перехват и подмена cookies
- `internal/logger/` — централизованное логирование
- `cmd/walletchecker/` — точка входа приложения

### Стек

- `github.com/go-resty/resty/v2` — HTTP клиент
- `github.com/tidwall/gjson` — парсинг JSON
- `github.com/joho/godotenv` — .env файлы
- `github.com/gotd/td/` — Telegram резолвер
- `github.com/dgraph-io/badger/v4` — локальная БД
- `github.com/playwright-community/playwright-go` — браузер автоматизация
- `fyne.io/fyne/v2` — GUI фреймворк
