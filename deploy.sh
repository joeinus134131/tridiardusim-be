#!/usr/bin/env bash
set -euo pipefail

repo_dir="/home/ubuntu/tridiardusim-be"
cd "$repo_dir"

exec 9>/var/lock/tridiardusim-deploy.lock
flock -n 9 || exit 0

git fetch --quiet origin main
current_revision="$(git rev-parse HEAD)"
target_revision="$(git rev-parse origin/main)"

if [[ "$current_revision" == "$target_revision" ]]; then
  exit 0
fi

git merge --ff-only origin/main
sudo docker compose build api
sudo docker compose up -d --no-deps api

for attempt in {1..20}; do
  if curl -fsS http://127.0.0.1:8080/health >/dev/null; then
    echo "Deployed $target_revision"
    exit 0
  fi
  sleep 3
done

echo "Deployment health check failed for $target_revision" >&2
exit 1
