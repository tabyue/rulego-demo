FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /rulego-demo .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /rulego-demo .
COPY rules/ ./rules/

EXPOSE 8080
ENV PORT=8080

CMD ["./rulego-demo"]
