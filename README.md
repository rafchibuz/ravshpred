# RAVSHANN Предложка

Локально запускаемый сервис предложений YouTube-видео: публичная лента, Twitch-вход,
голоса, очередь модерации, отметка «Отсмотрено» и отдельная панель основателя.

## Быстрый локальный запуск

Нужен Docker Desktop:

```powershell
docker compose -f deploy\compose.yaml up -d --build
```

После запуска:

- `http://127.0.0.1:8088/` — публичная гостевая версия;
- `http://127.0.0.1:8088/?demo=1` — локальный QA-переключатель ролей;
- `http://127.0.0.1:8088/?demo=1&role=moderator#/moderation` — модерация;
- `http://127.0.0.1:8088/?demo=1&role=owner#/owner` — управление;
- `http://127.0.0.1:8088/#/founder-access` — непубличный вход основателя.

Состояние контейнеров:

```powershell
docker compose -f deploy\compose.yaml ps
docker compose -f deploy\compose.yaml logs --tail=100
```

## Конфигурация

Скопируйте `.env.example` в `.env`. Для настоящего Twitch-входа укажите
`TWITCH_CLIENT_ID`, `TWITCH_CLIENT_SECRET` и зарегистрируйте callback:

`http://localhost:8088/api/auth/twitch/callback`

Пользователь сначала должен один раз войти через Twitch. После этого основатель
может назначить его модератором по точному Twitch-нику.

Для закрытого входа основателя задаётся только SHA-256 хеш секрета:

```powershell
$secret = Read-Host 'Founder secret'
$bytes = [Text.Encoding]::UTF8.GetBytes($secret)
$hash = [Security.Cryptography.SHA256]::HashData($bytes)
[Convert]::ToHexString($hash).ToLowerInvariant()
```

Полученное значение сохраните как `OWNER_BOOTSTRAP_TOKEN_HASH`. Сам секрет в `.env`
не хранится. Маршрут отсутствует в навигации и при пустом хеше отвечает 404.

Для реальных просмотров, лайков и длительности YouTube нужен `YOUTUBE_API_KEY`.
Без ключа заголовок, автор и превью берутся через YouTube oEmbed, а числовые метрики
остаются нулевыми.

`ALLOW_DEV_AUTH=true` и `VITE_QA_MODE=1` допустимы только локально. Перед публикацией
обязательно установите оба значения в `false`/`0`.

## Проверка

```powershell
pnpm test
pnpm build
cd backend
$env:GOCACHE="$PWD\..\tmp\go-build"
go test ./...
go vet ./...
```

Архитектура доступа описана в `docs/auth-architecture.md`, контейнеры —
в `deploy/README.md`.
