FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o /out/server ./cmd/server

# Alpine rather than distroless: the backend runs FFmpeg to transcode tracks
# for player downloads.
FROM alpine:3.21
RUN apk add --no-cache ca-certificates ffmpeg su-exec
COPY --from=build /out/server /app/server
COPY entrypoint.sh /app/entrypoint.sh
# Starts as root only to fix volume ownership, then drops to PUID:PGID.
VOLUME ["/cache", "/config"]
EXPOSE 8080
ENTRYPOINT ["/app/entrypoint.sh"]
