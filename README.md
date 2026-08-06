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
- `http://127.0.0.1:8088/?demo=1` — единый адрес локальной проверки. Он сразу
  открывает главную в роли владельца, а роль можно сменить кнопкой `QA` сверху.

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

Twitch-пользователь с ником из `OWNER_TWITCH_LOGIN` автоматически получает роль
`owner` при входе. По умолчанию это `rafchibiskus`. Отдельного закрытого входа
основателя нет.

Для реальных просмотров, лайков и длительности YouTube нужен `YOUTUBE_API_KEY`.
Без ключа заголовок, автор и превью берутся через YouTube oEmbed, а вместо
неизвестных числовых метрик интерфейс показывает `—`.

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
