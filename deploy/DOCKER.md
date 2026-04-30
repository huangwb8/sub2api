# Sub2API Docker Image

Sub2API is an AI API Gateway Platform for distributing and managing AI product subscription API quotas.

## Quick Start

```bash
docker run -d \
  --name sub2api \
  -p 8080:8080 \
  -e DATABASE_URL="postgres://user:pass@host:5432/sub2api" \
  -e REDIS_URL="redis://host:6379" \
  weishaw/sub2api:latest
```

## Docker Compose

```yaml
version: '3.8'

services:
  sub2api:
    image: weishaw/sub2api:latest
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://postgres:postgres@db:5432/sub2api?sslmode=disable
      - REDIS_URL=redis://redis:6379
    depends_on:
      - db
      - redis

  db:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=sub2api
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

## Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `DATABASE_URL` | PostgreSQL connection string | Yes | - |
| `REDIS_URL` | Redis connection string | Yes | - |
| `PORT` | Server port | No | `8080` |
| `GIN_MODE` | Gin framework mode (`debug`/`release`) | No | `release` |

## Supported Architectures

- Primary release path: `linux/amd64`
- Optional manual backfill path: `linux/arm64`

## Tags

- `latest` - Latest stable release
- `x.y.z` - Specific version
- `x.y` - Latest patch of minor version
- `x` - Latest minor of major version

## Automated Publishing

Sub2API publishes Docker images from the GitHub release pipeline, using `backend/cmd/server/VERSION` as the version source.
The default release path now prioritizes getting the Docker Hub `linux/amd64` image online first.

If you need a step-by-step GitHub repository setup guide, read:
- `docs/GITHUB_REPOSITORY_SETUP_TUTORIAL.md`

### Related Workflows

- `create-release.yml`: reads `backend/cmd/server/VERSION`, creates the annotated release tag, and triggers the main release job
- `release.yml`: publishes the primary Docker Hub `linux/amd64` image immediately; `workflow_dispatch` supports `publish_profile` for optional GHCR amd64 mirroring
- `publish-release-images.yml`: manual backfill workflow for optional GHCR and multi-arch publishing, controlled by `publish_profile`

### Registries

- Docker Hub: `weishaw/sub2api`
- GitHub Container Registry: `ghcr.io/<owner>/sub2api`

### Expected Maintenance Flow

1. Update `backend/cmd/server/VERSION`
2. Update `CHANGELOG.md`
3. Run `make verify-release-automation`
4. Create or push the release tag so `release.yml` can publish the primary Docker Hub amd64 image
5. Run `publish-release-images.yml` only when you explicitly want to backfill GHCR or multi-arch tags

If Docker Hub credentials are not configured, the primary release workflow fails fast because Docker Hub amd64 is the first-class target.

## Links

- [GitHub Repository](https://github.com/weishaw/sub2api)
- [Documentation](https://github.com/weishaw/sub2api#readme)
