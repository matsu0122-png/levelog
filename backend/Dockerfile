FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

FROM alpine:3.23
RUN apk add --no-cache tzdata
RUN adduser -D -u 10001 appuser
COPY --from=build /out/api /usr/local/bin/api
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://127.0.0.1:8080/health/live || exit 1
ENTRYPOINT ["/usr/local/bin/api"]
