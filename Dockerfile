FROM golang:1.26-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/mall .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/mall /mall

EXPOSE 8080

ENTRYPOINT ["/mall"]
