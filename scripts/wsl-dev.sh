#!/usr/bin/env bash
# Локальный запуск без Docker в WSL (Debian/Ubuntu): PostgreSQL + seed + сервер.
#
#   bash scripts/wsl-dev.sh setup   # один раз: запустить Postgres, создать роль/БД v2 (нужен sudo)
#   bash scripts/wsl-dev.sh seed    # пересоздать демо-пользователей (очищает таблицы пользователей;
#                                   # правки вопросов/заданий из админки сохраняются, их сброс: seed -reset-content)
#   bash scripts/wsl-dev.sh run     # собрать и запустить сервер на :3000
#                                   # админка: /admin (ADMIN_LOGIN/ADMIN_PASSWORD, по умолчанию admin / admin12345)
#                                   # кабинет агента: /partner (код + пароль; у засеянных агентов пароль agent12345)
#   bash scripts/wsl-dev.sh test    # все тесты, включая тесты с PostgreSQL
#
# Переменные: PGPORT (по умолчанию 5433 — порт кластера в `pg_lsclusters`), PORT (3000).
set -euo pipefail

cd "$(dirname "$0")/.."
export PATH="$PATH:/usr/local/go/bin"
PGPORT="${PGPORT:-5433}"
export DATABASE_URL="${DATABASE_URL:-postgres://v2:v2@localhost:${PGPORT}/v2?sslmode=disable}"

case "${1:-}" in
setup)
	sudo service postgresql start
	sudo -u postgres psql -p "$PGPORT" -tAc "SELECT 1 FROM pg_roles WHERE rolname='v2'" | grep -q 1 ||
		sudo -u postgres psql -p "$PGPORT" -c "CREATE ROLE v2 LOGIN PASSWORD 'v2'"
	sudo -u postgres psql -p "$PGPORT" -tAc "SELECT 1 FROM pg_database WHERE datname='v2'" | grep -q 1 ||
		sudo -u postgres psql -p "$PGPORT" -c "CREATE DATABASE v2 OWNER v2"
	;;
seed)
	shift
	go run ./cmd/seed -reset "$@"
	;;
run)
	go build -o /tmp/v2server ./cmd/server
	DEV_ALLOW_FAKE_AUTH=true PORT="${PORT:-3000}" 		ADMIN_LOGIN="${ADMIN_LOGIN:-admin}" ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin12345}" 		exec /tmp/v2server
	;;
test)
	TEST_DATABASE_URL="$DATABASE_URL" go test ./...
	;;
*)
	sed -n '2,14p' "$0"
	exit 1
	;;
esac
