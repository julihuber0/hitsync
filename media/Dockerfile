FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o /out/media ./cmd/media

FROM alpine:3.21
RUN apk add --no-cache ca-certificates ffmpeg \
    && adduser -D -u 65532 appuser \
    && install -d -o appuser -g appuser /cache
COPY --from=build /out/media /app/media
USER appuser
VOLUME /cache
EXPOSE 8090
ENTRYPOINT ["/app/media"]
