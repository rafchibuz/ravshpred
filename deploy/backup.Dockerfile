FROM postgres:16-alpine

COPY deploy/scripts/backup.sh /usr/local/bin/backup.sh
COPY deploy/scripts/verify-backup.sh /usr/local/bin/verify-backup.sh
COPY deploy/scripts/restore-backup.sh /usr/local/bin/restore-backup.sh

RUN sed -i 's/\r$//' /usr/local/bin/backup.sh /usr/local/bin/verify-backup.sh /usr/local/bin/restore-backup.sh \
    && chmod +x /usr/local/bin/backup.sh /usr/local/bin/verify-backup.sh /usr/local/bin/restore-backup.sh

ENTRYPOINT ["/bin/sh", "/usr/local/bin/backup.sh"]
