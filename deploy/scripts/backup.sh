#!/usr/bin/env bash
# ==============================================================================
# GOGIGANET BROADBAND - AUTOMATED SQLITE HOT BACKUP SCRIPT
# Path: /var/www/isp/deploy/scripts/backup.sh
# Dipasang via Crontab harian: 0 2 * * * /var/www/isp/deploy/scripts/backup.sh
# ==============================================================================

set -euo pipefail

# Directory paths
APP_DIR="/var/www/isp"
DB_FILE="${APP_DIR}/onboarding.db"
BACKUP_DIR="/var/backups/gogiga-isp"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_TARGET="${BACKUP_DIR}/onboarding_${TIMESTAMP}.sqlite3"

# Create backup directory if not exists
mkdir -p "${BACKUP_DIR}"
chmod 700 "${BACKUP_DIR}"

echo "[INFO] [$(date)] Memulai proses pencadangan database SQLite GOGIGANET..."

# 1. Hot online backup menggunakan SQLite command-line (Aman tanpa mematikan aplikasi)
if [ -f "${DB_FILE}" ]; then
    sqlite3 "${DB_FILE}" ".backup '${BACKUP_TARGET}'"
    
    # 2. Kompresi gzip untuk menghemat disk storage
    gzip -9 "${BACKUP_TARGET}"
    
    # Kunci hak akses arsip cadangan
    chmod 600 "${BACKUP_TARGET}.gz"
    echo "[SUCCESS] [$(date)] Database berhasil dicadangkan ke: ${BACKUP_TARGET}.gz"
else
    echo "[ERROR] [$(date)] File database ${DB_FILE} tidak ditemukan!"
    exit 1
fi

# 3. Rotasi cadangan: Hapus backup yang lebih lama dari 14 hari
find "${BACKUP_DIR}" -name "onboarding_*.sqlite3.gz" -type f -mtime +14 -exec rm -f {} \;
echo "[INFO] [$(date)] Rotasi arsip selesai. Cadangan lama (>14 hari) telah dibersihkan."
