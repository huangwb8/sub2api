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

- `linux/amd64`
- `linux/arm64`

## Tags

- `latest` - Latest stable release
- `x.y.z` - Specific version
- `x.y` - Latest patch of minor version
- `x` - Latest minor of major version

## Automated Publishing

Sub2API publishes Docker images from the GitHub release pipeline, using `backend/cmd/server/VERSION` as the version source.

If you need a step-by-step GitHub repository setup guide, read:
- `docs/GITHUB_REPOSITORY_SETUP_TUTORIAL.md`

### Related Workflows

- `create-release.yml`: reads `backend/cmd/server/VERSION`, creates the annotated release tag, and triggers the main release job
- `release.yml`: builds release artifacts and publishes the primary image set
- `publish-release-images.yml`: re-checks the latest GitHub release and backfills missing image tags automatically

### Registries

- Docker Hub: `weishaw/sub2api`
- GitHub Container Registry: `ghcr.io/<owner>/sub2api`

### Expected Maintenance Flow

1. Update `backend/cmd/server/VERSION`
2. Update `CHANGELOG.md`
3. Run `make verify-release-automation`
4. Trigger `create-release.yml`
5. Use `publish-release-images.yml` only when you need to backfill or repair image tags

If Docker Hub credentials are not configured, the automation continues to publish to GitHub Container Registry and skips Docker Hub gracefully.

## Links

- [GitHub Repository](https://github.com/weishaw/sub2api)
- [Documentation](https://github.com/weishaw/sub2api#readme)
