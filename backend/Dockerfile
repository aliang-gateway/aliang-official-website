# Stage 1: Build the Go binary
# golang:1.25-alpine is not yet available on Docker Hub,
# so we use 1.24 with GOTOOLCHAIN=auto to auto-download the required toolchain.
FROM golang:1.24-alpine AS builder
WORKDIR /app

ENV GOTOOLCHAIN=auto

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /server .

# Stage 2: Production image
FROM alpine:latest AS runner
WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /server ./server
COPY --from=builder /app/config.example.yaml ./config.example.yaml

USER appuser

EXPOSE 8080

CMD ["./server"]
