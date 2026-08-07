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

## Резервные копии PostgreSQL

Сервис `backup` создаёт проверенный архив PostgreSQL сразу после запуска, затем
раз в сутки. По умолчанию сохраняются 7 ежедневных и 4 воскресных копии.
Архивы и SHA-256 checksums находятся в `backups/daily` и `backups/weekly` и не
попадают в Git.

Посмотреть состояние и созданные файлы:

```bash
docker compose -f deploy/compose.yaml logs --tail=50 backup
docker compose -f deploy/compose.yaml exec backup ls -lh /backups/daily
```

Проверить выбранную копию без изменения базы:

```bash
docker compose -f deploy/compose.yaml exec backup \
  verify-backup.sh /backups/daily/ravshann-YYYYMMDDTHHMMSSZ.dump
```

Для production задайте `BACKUP_HOST_DIR` на отдельный примонтированный диск или
каталог, который синхронизируется во внешнее хранилище. Копия только на диске
того же сервера не защищает от потери VPS.

Восстановление намеренно требует отдельный адрес целевой базы и явное
`RESTORE_CONFIRM=RESTORE`. Сначала восстанавливайте архив во временную БД.
Команда очищает объекты именно в указанной целевой базе:

```bash
docker compose -f deploy/compose.yaml exec \
  -e TARGET_DATABASE_URL=postgres://user:password@host:5432/test_restore \
  -e RESTORE_CONFIRM=RESTORE \
  backup restore-backup.sh /backups/daily/ravshann-YYYYMMDDTHHMMSSZ.dump
```
