# Backend deployment

Production runs from `/home/ubuntu/tridiardusim-be` with Docker Compose. Caddy
routes `https://api.idnmakerspace.com` to the `tridiardusim-api` container on
the shared `n8n_default` Docker network.

## Automatic updates

`tridiardusim-deploy.timer` checks `origin/main` every two minutes. When the
remote revision changes, `deploy.sh` performs a fast-forward-only update,
builds the API image, recreates the API container, and verifies `/health`.
Project JSON files remain in the `tridiardusim-be_project_data` Docker volume.

Normal release flow:

```bash
git switch main
git pull --ff-only
# edit and test
git push origin main
```

Useful production checks:

```bash
sudo systemctl status tridiardusim-deploy.timer
sudo journalctl -u tridiardusim-deploy.service -n 100 --no-pager
sudo docker compose ps
curl -fsS https://api.idnmakerspace.com/health
```

To deploy immediately instead of waiting for the timer:

```bash
sudo systemctl start tridiardusim-deploy.service
```

The deployment intentionally refuses non-fast-forward Git history. Do not
force-push `main`; merge or revert with a new commit instead.
