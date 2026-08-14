FROM golang

WORKDIR /lanchat
COPY . .
RUN go mod tidy
RUN go build ./cmd/lanchat

CMD ["/lanchat/lanchat"]