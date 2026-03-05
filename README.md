# tg-monitor

Утилита на Go: мониторит Telegram-чаты и форумы по ключевым словам, сохраняет совпадения в PostgreSQL. Умеет экспортировать результаты в Excel.

---

## Что умеет

- Вступает в обычные каналы/группы по username или invite-ссылке
- Разворачивает **форум-группы** (с топиками, как `t.me/gantsevichi_obyavleniya`) — автоматически обходит все вложенные темы
- Ищет ключевые слова в новых сообщениях (регистр не важен, ищет целые слова — «куплю» не сработает на «закуплю»)
- Сохраняет в PostgreSQL с дедупликацией — повторные запуски не дублируют записи
- Экспортирует все найденные сообщения в Excel со статистикой

---

## Требования

Нужен только **Docker** — Go и PostgreSQL устанавливать не нужно.

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (включает docker compose)

---

## Быстрый старт

### 1. Получить Telegram API credentials

1. Зайти на [my.telegram.org/apps](https://my.telegram.org/apps)
2. Войти под своим номером телефона
3. Нажать «Create application» → скопировать `App api_id` и `App api_hash`

> ⚠️ Это credentials реального аккаунта, не бота. Утилита работает от имени вашего аккаунта. Рекомендуется использовать отдельный аккаунт для мониторинга.

### 2. Настроить конфиг

В проекте два конфигурационных файла — оба нужно заполнить одинаково, отличается только `database.url`:

**`config.docker.yml`** — используется при запуске через Docker (основной способ):

```yaml
telegram:
  app_id: 12345678           # ← ваш App api_id
  app_hash: "abc123..."      # ← ваш App api_hash
  phone: "+79001234567"      # ← ваш номер телефона

database:
  url: "postgres://tg_monitor:tg_monitor_pass@postgres:5432/tg_monitor?sslmode=disable"
  # ↑ не менять — это адрес внутреннего контейнера

keywords:
  - куплю
  - продам
  - ищу

sources:
  # Форум-группы — скрипт сам обойдёт все топики внутри
  forums:
    - https://t.me/gantsevichi_obyavleniya

  # Обычные каналы и группы
  chats:
    - some_channel

export:
  dir: "/app/exports"        # не менять
```

Нужно заполнить только секцию `telegram`, `keywords` и `sources` — остальное оставить как есть.

### 3. Запустить

```bash
docker compose up --build
```

**При первом запуске** программа спросит код подтверждения из Telegram:

```
Введите код из Telegram: 12345
```

После ввода кода сессия сохраняется в папке `.session/` — при следующих запусках авторизация не нужна.

### 4. Остановить

```
Ctrl+C
```

---

## Экспорт в Excel

После того как монитор поработал и собрал сообщения:

```bash
docker compose run --rm export
```

Файл `export_YYYY-MM-DD_HH-MM-SS.xlsx` появится в папке `exports/` рядом с проектом.

Файл содержит два листа:
- **Результаты** — таблица с фильтрами: дата, чат, топик, отправитель, ключевое слово, текст
- **Статистика** — сводка по ключевым словам и чатам

---

## Конфигурация — полное описание

```yaml
telegram:
  app_id: 12345678           # App api_id с my.telegram.org (число)
  app_hash: "строка"         # App api_hash с my.telegram.org
  phone: "+7..."             # Номер телефона с кодом страны

database:
  url: "postgres://..."      # DSN подключения к PostgreSQL

keywords:                    # Ключевые слова (регистр не важен, целые слова)
  - слово1
  - слово2

sources:
  forums:                    # Форум-группы с топиками
    - https://t.me/username  # По username — скрипт вступит и обойдёт все темы
    - https://t.me/+hash     # По invite-ссылке тоже работает

  chats:                     # Обычные каналы и группы
    - username               # Просто username, без @ и без ссылки
    - https://t.me/+hash     # Или invite-ссылка

export:
  dir: "./exports"           # Папка для сохранения xlsx файлов
```

---

## Запуск без Docker (если установлен Go 1.22+)

```bash
# Заполнить config.yml (там database.url указывает на локальный PostgreSQL)
go run ./cmd/monitor          # запустить мониторинг
go run ./cmd/export           # экспортировать в xlsx → папка ./exports
```

---

## Обновление до новой версии

```bash
git pull                     # или скачать новый архив
docker compose up --build    # пересобрать образ и перезапустить
```

База данных и сессия сохраняются — ничего не теряется.

---

## Структура проекта

```
tg-monitor/
├── cmd/
│   ├── monitor/main.go      # команда: мониторинг
│   └── export/main.go       # команда: экспорт в xlsx
├── internal/
│   ├── config/              # загрузка config.yml
│   ├── db/                  # PostgreSQL (pgx)
│   ├── tg/                  # Telegram MTProto (gotd/td)
│   │   ├── client.go        # авторизация, сессия
│   │   ├── resolver.go      # вступление в чаты, разворот форумов
│   │   └── monitor.go       # приём и фильтрация сообщений
│   └── exporter/            # генерация xlsx
├── config.yml               # конфиг для локального запуска (Go напрямую)
├── config.docker.yml        # конфиг для Docker (database.url → контейнер postgres)
├── docker-compose.yml
└── Dockerfile
```

---

## Частые вопросы

**Могут ли заблокировать аккаунт?**
Telegram следит за подозрительной активностью. Не добавляйте сразу сотни источников — утилита делает паузы между вступлениями (600 мс), но лучше не превышать 20–30 источников за раз.

**Где хранится сессия?**
В папке `.session/session.json`. Держите её в безопасности — это авторизация вашего аккаунта в Telegram.

**Как посмотреть данные напрямую в БД?**
```bash
docker compose exec postgres psql -U tg_monitor -d tg_monitor -c \
  "SELECT chat_title, keyword, sender_name, sent_at, left(text,100) FROM messages ORDER BY sent_at DESC LIMIT 20;"
```