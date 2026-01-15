FROM golang:1.25 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN mkdir -p /out && CGO_ENABLED=0 go build -o /out/sling .

FROM alpine:3.19

WORKDIR /app

COPY --from=builder /out/sling /app/sling

ENTRYPOINT ["/app/sling"]
CMD ["serve"]
