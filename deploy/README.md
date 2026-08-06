# Локальный полный стек

1. Скопируйте `.env.example` в `.env`.
2. Укажите Twitch Client ID и Secret. Без них Twitch не запустит OAuth.
3. Запустите:

```bash
docker compose -f deploy/compose.yaml up --build
```

Сайт будет доступен на `http://localhost:8088`, readiness API —
`http://localhost:8088/health/ready`.

Роли выдаются только через Twitch-сессию. Тестового входа и заголовков,
подменяющих пользователя, в приложении нет.
