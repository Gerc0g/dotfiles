# Plane Agent Board Pilot

Plane is the pilot board/control-plane for agent work across repositories that
live on different Git providers. Git providers remain the evidence layer
(branch, commit, PR/MR, CI); Plane owns the task state, acceptance criteria, and
review workflow.

This pilot uses the official Plane Community Edition `setup.sh` flow. It keeps
Plane isolated from the main dev-stack data services for the pilot, while the
Plane proxy joins the existing `dev-stack_default` Docker network so the
existing Traefik instance can route `board.test` to Plane.

## Shape

```text
Codex / Claude
      |
      v
Plane board  <http://board.test>
      |
      +-- GitHub / GitLab / Gitea links
      +-- branch / commit / PR-MR / CI evidence
      +-- tracker-guard later for Done policy
```

## Server Layout

```text
/root/plane-selfhost/setup.sh
/root/plane-selfhost/plane-app/docker-compose.yaml
/root/plane-selfhost/plane-app/plane.env
```

The generated `plane.env` pilot values:

```dotenv
APP_DOMAIN=board.test
APP_RELEASE=v1.3.1
LISTEN_HTTP_PORT=8087
LISTEN_HTTPS_PORT=8443
WEB_URL=http://board.test
CORS_ALLOWED_ORIGINS=http://board.test,http://board.100.73.117.50.sslip.io
SITE_ADDRESS=:80
```

The generated compose is patched so the `proxy` service has:

```yaml
container_name: plane-proxy
networks:
  - default
  - dev-stack

networks:
  dev-stack:
    external: true
    name: dev-stack_default
```

## Start

```bash
cd /root/plane-selfhost
./setup.sh start
```

Then open:

```text
http://board.test
http://board.100.73.117.50.sslip.io
```

## Agent Workflow

Use Plane as the source of truth for:

- task type: epic, feature, task, bug, subtask;
- status: backlog, ready, in-progress, blocked, review, done;
- agent owner;
- acceptance criteria;
- definition of done;
- links to repo, branch, commit, PR/MR, and CI.

Agents may move work to `review`. `done` should later be controlled by a
separate guard/human approval flow after PR/MR and CI evidence is present.

## Notes

- This is a pilot, not the final hardened setup.
- Plane owns its own pilot Postgres, Redis, MinIO, and RabbitMQ volumes.
- The existing dev-stack Traefik only routes to `plane-proxy`.
- For production, replace dev credentials and add backups before relying on it.
