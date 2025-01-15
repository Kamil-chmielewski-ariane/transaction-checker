FROM golang:1.23.1 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
RUN go build -o transaction_checker .

FROM ubuntu:23.10

WORKDIR /app

ARG LOG_FILE_PATH
RUN mkdir -p ${LOG_FILE_PATH}

COPY --from=builder /app/transaction_checker /usr/local/bin/transaction_checker


EXPOSE ${PORT}

CMD ["/usr/local/bin/transaction_checker"]
