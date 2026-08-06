# Локальный полный стек

1. Скопируйте `.env.example` в `.env`.
2. Для локальной проверки ключи Twitch, YouTube и Turnstile можно оставить пустыми.
3. Запустите:

```bash
docker compose -f deploy/compose.yaml up --build
```

Сайт будет доступен на `http://localhost:8088`, readiness API —
`http://localhost:8088/health/ready`.

`ALLOW_DEV_AUTH=true` разрешён только для локальной разработки. Перед любым
публичным запуском установите `ALLOW_DEV_AUTH=false` и настройте реальные OAuth,
Turnstile и owner passkey.

