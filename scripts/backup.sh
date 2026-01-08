#!/bin/sh
# PostgreSQL backup script for Docker

set -e

# Configuration
BACKUP_DIR="/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/planbot_${TIMESTAMP}.sql"
RETENTION_DAYS=7

# Create backup directory if not exists
mkdir -p "${BACKUP_DIR}"

# Perform backup
echo "Starting backup at $(date)"
pg_dump > "${BACKUP_FILE}"

# Compress backup
gzip "${BACKUP_FILE}"
echo "Backup completed: ${BACKUP_FILE}.gz"

# Remove old backups
echo "Removing backups older than ${RETENTION_DAYS} days..."
find "${BACKUP_DIR}" -name "planbot_*.sql.gz" -mtime +${RETENTION_DAYS} -delete

# List current backups
echo "Current backups:"
ls -lh "${BACKUP_DIR}"

echo "Backup process completed at $(date)"

