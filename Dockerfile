# syntax=docker/dockerfile:1.7

FROM golang:1.24-alpine AS builder

RUN apk add --no-cache ca-certificates tzdata protobuf protobuf-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/go-task/task/v3/cmd/task@latest \
    && PATH="$(go env GOPATH)/bin:$PATH" task proto:install proto:gen

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/server /server

EXPOSE 50051

ENTRYPOINT ["/server"]
