FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.work go.work
COPY shared shared
COPY services services

WORKDIR /src/services/driver-matching-service
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 go build -o /out/driver-matching-service ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/driver-matching-service /usr/local/bin/driver-matching-service
ENTRYPOINT ["/usr/local/bin/driver-matching-service"]
