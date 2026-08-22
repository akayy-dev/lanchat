# LANChat - P2P Chat Protocol utilizing Multicast and TCP

A simple, real-time chat application for local area networks (LAN). Discover and chat with other users on your network without any server setup.

## Features

- **Automatic Peer Discovery**: Finds other LANChat users on your network automatically using multicast
- **Real-time Messaging**: Send and receive messages instantly over TCP
- **Terminal UI**: Clean, colorful chat interface in your terminal
- **User Presence**: See when users join and leave the chat
- **No Server Required**: Fully peer-to-peer - just run and chat

## How It Works

LANChat uses a two-stage networking approach:

1. **Discovery (Multicast)**: Each client broadcasts their presence and contact information over the local network using multicast UDP. This allows automatic peer discovery without manual configuration.

2. **Messaging (TCP)**: Once peers are discovered, messages are sent directly between users using TCP connections for reliable delivery.

## Requirements

- Go 1.25.0 or higher
- Local network with multicast support

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
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