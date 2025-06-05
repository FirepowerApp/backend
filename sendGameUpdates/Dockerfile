FROM golang:1.23.3 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM gcr.io/distroless/static

WORKDIR /
COPY --from=builder /app/main .

ENV PORT=8080
EXPOSE 8080

CMD ["/main"]