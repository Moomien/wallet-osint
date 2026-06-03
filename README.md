## Wallet Doxxer

Пробиваешь крипто кошельки, вытаскиваешь твиттеры, телеграм ники, прогоняешь через Grok AI и пушишь всё в Google Sheets. Всё автоматом, без лишних движений.

### Что умеет

- Читает кошельки из `addresses.txt`
- Проверяет уникальность в BadgerDB (не прогоняет одно и то же дважды)
- Параллельно пробивает по Arkham API и тянет привязанные Twitter аккаунты
- Резолвит твиттер ники в телеграм юзернеймы
- Прогоняет каждый твиттер аккаунт через Grok AI (анализ профиля, активность, связи)
- Публикует результаты от Grok в GitHub Gist
- Собирает все данные в Google Sheets таблицу с колонками: Wallet → Twitter → Telegram → Gist
- Сохраняет бэкап в `result.txt`

### Как запустить

1. Настрой `.env` (смотри `.env.example`):

```
cookie=...твой cookie из intel.arkm.com...
```

2. Подготовь файлы:
   - `addresses.txt` — список кошельков (по одному на строку)
   - `proxy.txt` (опционально) — прокси в формате `http://логин:пароль@адрес:порт`
   - `grokclient/sessions/` — сессии Grok (файлы с куками)
   - `gsheets/credentials.json` — креды для Google Sheets API

3. Запусти:

```bash
go run main.go          # без прокси
go run main.go proxy    # с прокси
```

### Что внутри

- `checker/` — модуль сбора твиттеров через Arkham
- `resolver/` — резолвинг твиттер → телеграм
- `grokclient/` — пул воркеров для Grok AI
- `gsheets/` — выгрузка в Google Sheets
- `storage/` — BadgerDB для отсева дубликатов
- `session/` — кеш и управление сессиями
- `interceptor/` — перехват и подмена cookies

### Стек

- `github.com/go-resty/resty/v2` — HTTP клиент
- `github.com/tidwall/gjson` — парсинг JSON
- `github.com/joho/godotenv` — .env файлы
- `github.com/gotd/td/` — Telegram резолвер
- `github.com/dgraph-io/badger/v4` — локальная БД
- `google.golang.org/api/sheets/v4` — Google Sheets API
- `github.com/go-rod/rod` — браузер автоматизация для сессий
