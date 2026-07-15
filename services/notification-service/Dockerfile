FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.work go.work
COPY shared shared
COPY services services

WORKDIR /src/services/notification-service
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 go build -o /out/notification-service ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/notification-service /usr/local/bin/notification-service
ENTRYPOINT ["/usr/local/bin/notification-service"]
