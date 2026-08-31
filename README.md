# LANChat - P2P Chat Application with Live Discovery and Secure Encryption
![screenshot of app](./doc/screenshot.png)
A simple, secure, and real-time chat application for local area networks that protects against packet sniffing.

## How It Works

The LANChat application consists of three parts:

1. **Discovery:** Clients broadcast their presence over multicast, this allows for peers to automatically discover one another.

2. **Messaging:**: Once peers are discovered, messages are sent directly between users using TCP connections for reliable delivery.

3. **Encryption:** When peers connect, they securely swap encryption keys with each user using Diffie-Hellman, ensuring secure communication from network monitoring tools.
## An important note:
Many public networks have security measures in place to block devices from communicating with each other, as such behavior in public spaces such as a libraries or airports may be unpredictable

## Installation

1. Clone the repository:
```bash
git clone https://github.com/akayy-dev/lanchat
cd LANChat
```

2. Install dependencies:
```bash
go mod download
```

3. Build and run:
```bash
go build -o lanchat cmd/lanchat/main.go
./lanchat
```