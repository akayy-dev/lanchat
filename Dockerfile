FROM golang:1.24.2-alpine

WORKDIR /app

# Copy your Go module files (if any)
COPY go.mod go.sum* ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build the chat app
RUN go build -o lanchat .

# Start interactively
ENTRYPOINT ["./lanchat"]

