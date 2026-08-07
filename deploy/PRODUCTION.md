# Развёртывание на VPS

Проект рассчитан на запуск рядом с уже работающими сервисами. Веб-интерфейс
публикуется только на `127.0.0.1:8088`, а внешний HTTPS-доступ предоставляет
Cloudflare Tunnel. Поэтому контейнеры не занимают публичные порты `80` и `443`.

## Требования

- Docker Engine и Docker Compose v2;
- Git;
- минимум 2 ГБ RAM и 2 ГБ swap;
- домен, добавленный в Cloudflare;
- Twitch OAuth-приложение с production callback;
- свободный локальный TCP-порт `8088`.

## Подготовка сервера

Перед системными изменениями создайте снимок VPS в панели провайдера. Если swap
на сервере отсутствует, создайте файл подкачки:

```bash
fallocate -l 2G /swapfile
chmod 600 /swapfile
mkswap /swapfile
swapon /swapfile
grep -qF '/swapfile none swap sw 0 0' /etc/fstab || \
  printf '%s\n' '/swapfile none swap sw 0 0' >> /etc/fstab
printf '%s\n' 'vm.swappiness=10' > /etc/sysctl.d/99-ravshpred-swap.conf
sysctl --system
```

Проверка:

```bash
free -h
swapon --show
```

## Установка приложения

```bash
git clone https://github.com/rafchibuz/ravshpred.git /opt/ravshpred
cd /opt/ravshpred
cp .env.example .env
chmod 600 .env
```

Сгенерируйте отдельный пароль PostgreSQL:

```bash
openssl rand -hex 32
```

Откройте `.env` через `nano .env` и заполните как минимум:

```dotenv
APP_BASE_URL=https://example.ru
FRONTEND_URL=https://example.ru
POSTGRES_PASSWORD=случайный_пароль
OWNER_TWITCH_LOGIN=rafchibiskus
OWNER_TWITCH_ID=866415576
TWITCH_CLIENT_ID=
TWITCH_CLIENT_SECRET=
TWITCH_REDIRECT_URL=https://example.ru/api/auth/twitch/callback
YOUTUBE_API_KEY=
LOCAL_HTTP_PORT=8088
CLOUDFLARE_TUNNEL_TOKEN=
```

Секреты нельзя добавлять в Git или передавать в сообщениях. Production callback
в настройках приложения Twitch должен полностью совпадать со значением
`TWITCH_REDIRECT_URL`.

## Локальная проверка на VPS

До настройки Cloudflare можно запустить основной стек без production-профиля:

```bash
docker compose --env-file .env -f deploy/compose.yaml up -d --build
docker compose --env-file .env -f deploy/compose.yaml ps
curl --fail http://127.0.0.1:8088/health/ready
```

Порт `8088` привязан только к loopback-интерфейсу и не доступен напрямую из
интернета.

## Cloudflare Tunnel

В панели Cloudflare Zero Trust создайте Tunnel типа Cloudflared. Для Public
Hostname задайте:

- hostname: production-домен;
- service type: HTTP;
- service URL: `http://frontend:80`.

Токен Tunnel сохраните в `CLOUDFLARE_TUNNEL_TOKEN`, затем запустите профиль:

```bash
docker compose --env-file .env -f deploy/compose.yaml --profile production up -d
docker compose --env-file .env -f deploy/compose.yaml --profile production ps
docker compose --env-file .env -f deploy/compose.yaml logs --tail=100 cloudflared
```

Cloudflared находится в одной внутренней Docker-сети с frontend, поэтому в
настройке Tunnel используется имя сервиса `frontend`, а не `localhost`.

## Обновление

Перед обновлением убедитесь, что резервная копия PostgreSQL создана:

```bash
cd /opt/ravshpred
docker compose --env-file .env -f deploy/compose.yaml logs --tail=50 backup
git pull --ff-only
docker compose --env-file .env -f deploy/compose.yaml --profile production up -d --build
curl --fail http://127.0.0.1:8088/health/ready
```

## Диагностика

```bash
docker compose --env-file .env -f deploy/compose.yaml --profile production ps
docker compose --env-file .env -f deploy/compose.yaml logs --tail=200
df -h /
free -h
```

Не выполняйте `docker system prune --volumes`: команда может удалить данные
других проектов на том же сервере.
