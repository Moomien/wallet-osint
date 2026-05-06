## Arkham checker

Утилита на Go, которая читает список EVM-адресов из `addresses.txt`, запрашивает Arkham API и вытаскивает привязанный Twitter (если есть). Результат сохраняется в `result.txt`.

### Как работает

- Загружает переменные окружения из `.env` (нужен `cookie` для запросов к Arkham).
- Читает `addresses.txt` и парсит адреса через `strings.Fields` (можно по строкам или через пробелы).
- Для каждого адреса параллельно делает запрос к `https://api.arkm.com/intelligence/address/<address>`.
- Ограничивает количество одновременных запросов семафором (буферизованный канал), чтобы не перегружать API.
- Достаёт поле `arkhamEntity.twitter` из JSON-ответа. Если Twitter не найден или запрос не удался после ретраев, адрес пропускается.
- Пишет собранные Twitter-хендлы в `result.txt`.

### Запуск

1. Создай `.env` рядом с `main.go`:

```
cookie=...твой cookie из intel.arkm.com...
```

2. Заполни `addresses.txt`:

```
0x...
0x...
```

3. Запусти:

```bash
go run .
```

### Использованные библиотеки

- `github.com/go-resty/resty/v2` — HTTP-клиент
- `github.com/tidwall/gjson` — быстрое чтение полей из JSON
- `github.com/joho/godotenv` — загрузка `.env`
