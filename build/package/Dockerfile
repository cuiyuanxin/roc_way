# rocway 多阶段构建：第一阶段编译，第二阶段 distroless 运行。
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go install github.com/google/wire/cmd/wire@latest \
 && wire ./internal/wire \
 && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/rocway ./cmd/rocway

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /out/rocway /app/rocway
COPY --from=builder /src/configs /app/configs
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/rocway"]
