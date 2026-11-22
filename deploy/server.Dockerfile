FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG GO_TAGS=""
RUN CGO_ENABLED=0 go build -tags "$GO_TAGS" -trimpath -o /out/ ./cmd/...

# The three web pages, built only for the "with-pages" target that `make up` uses
# (compose.ports.yaml names the "webpages" context). The test build never reaches this.
FROM node:22-alpine AS pages
COPY --from=webpages . /web
RUN set -e; for p in app control-panel notification-app; do \
      cd /web/$p; npm ci --no-audit --no-fund; VITE_API=network npx vite build --outDir /out/$p --emptyOutDir; \
    done

FROM alpine:3.21 AS part
RUN adduser -D -u 10001 part
COPY --from=build /out/ /usr/local/bin/
USER part
EXPOSE 8080

FROM part AS with-pages
COPY --from=pages /out/app /web/app
COPY --from=pages /out/control-panel /web/control
COPY --from=pages /out/notification-app /web/notifications

FROM part
