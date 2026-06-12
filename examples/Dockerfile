# syntax=docker/dockerfile:1

ARG FRANKENPHP_VERSION=v1.12.3
ARG CBROTLI_VERSION=v1.0.1

FROM dunglas/frankenphp:builder AS builder

ARG FRANKENPHP_VERSION
ARG CBROTLI_VERSION

COPY --from=caddy:builder /usr/bin/xcaddy /usr/bin/xcaddy
COPY module /src/queue/module

RUN CGO_ENABLED=1 \
    XCADDY_SETCAP=1 \
    XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx,nowatcher" \
    CGO_CFLAGS="-D_GNU_SOURCE $(php-config --includes)" \
    CGO_CPPFLAGS="$(php-config --includes)" \
    CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
    xcaddy build \
        --output /usr/local/bin/frankenphp \
        --with github.com/dunglas/frankenphp@${FRANKENPHP_VERSION} \
        --with github.com/dunglas/frankenphp/caddy@${FRANKENPHP_VERSION} \
        --with github.com/dunglas/caddy-cbrotli@${CBROTLI_VERSION} \
        --with github.com/y-l-g/queue/module=./src/queue/module

FROM dunglas/frankenphp AS runtime

COPY --from=builder /usr/local/bin/frankenphp /usr/local/bin/frankenphp
WORKDIR /app
