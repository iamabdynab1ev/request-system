FROM golang:latest

WORKDIR /app

COPY . .

RUN go install github.com/pressly/goose/v3/cmd/goose@latest

ENV PATH="$PATH:/go/bin" 

CMD ["goose", "-dir", "database/migrations", "postgres", "postgres://postgres:postgres@postgresql:5432/request-system?sslmode=disable", "up"]