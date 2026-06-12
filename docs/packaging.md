# Packaging And Compatibility

This project ships source packages and a FrankenPHP module build recipe. It does
not publish a separate worker daemon.

## Compatibility Matrix

| Component | Supported version | Notes |
| --- | --- | --- |
| FrankenPHP | 1.12.x | The example build pins `dunglas/frankenphp:1.12.3` and `github.com/dunglas/frankenphp@v1.12.3`. |
| Caddy | 2.11.x | Pulled through FrankenPHP's Caddy module dependency. |
| PHP | 8.5 ZTS | The module is built against FrankenPHP's ZTS PHP runtime. |
| Redis | 6.2+ | Required for Streams consumer groups and `XAUTOCLAIM`. |
| Laravel | 13.x | Provided by `pogo/laravel-queue`. |
| Symfony | 8.x | Provided by `pogo/symfony-queue`. |

Keep the application image, Composer package versions, and module build pinned
to the same release. Do not mix a newly built module with older framework
packages unless the changelog says that combination is supported.

## Docker Image

Use [examples/Dockerfile](../examples/Dockerfile) as the blessed image recipe:

```bash
docker build \
  -f examples/Dockerfile \
  --build-arg FRANKENPHP_IMAGE_VERSION=1.12.3 \
  --build-arg FRANKENPHP_VERSION=v1.12.3 \
  --build-arg CBROTLI_VERSION=v1.0.1 \
  -t ghcr.io/your-org/frankenphp-pogo-queue:2.0.0 .
```

For production, pin the base images by digest in your downstream Dockerfile and
build separate `linux/amd64` and `linux/arm64` images in CI.

Minimal release image checks:

```bash
docker run --rm ghcr.io/your-org/frankenphp-pogo-queue:2.0.0 frankenphp version
docker run --rm ghcr.io/your-org/frankenphp-pogo-queue:2.0.0 php -v
```

Then run an application smoke test that dispatches a job, observes
`pogo_queue_status()`, and verifies the job is acknowledged.

## Release Checksums

Generate SHA-256 checksums for every published binary, archive, and image
metadata artifact:

```bash
scripts/release-checksums.sh dist/*
```

Publish `SHA256SUMS` next to the release artifacts and verify it before
promoting a release:

```bash
sha256sum -c SHA256SUMS
```

## Release Checklist

1. Run the Go module tests with the PHP 8.5 ZTS FrankenPHP build.
2. Run Laravel PHPUnit, Symfony PHPUnit, and PHPStan for both packages.
3. Run Redis-gated tests with `POGO_REDIS_URL` against Redis 6.2 or newer.
4. Build the Docker image for every target architecture.
5. Smoke test dispatch, ack, release, fail, retry-failed, and purge-failed paths.
6. Generate and publish `SHA256SUMS`.
7. Tag the module and Composer package release from the same commit.
