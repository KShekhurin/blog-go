FROM golang:1.26-trixie AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

RUN go install github.com/pressly/goose/v3/cmd/goose@v3.27.2

COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /src/bin .

FROM alpine:3.20 AS web

RUN apk --no-cache add ca-certificates
WORKDIR /app

COPY --from=build /src/bin .
CMD ["./bin"]