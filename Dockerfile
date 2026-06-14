FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o provider ./cmd/provider

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /app/provider /app/provider
COPY --from=builder /app/web /app/web
ENTRYPOINT ["/app/provider"]
