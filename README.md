## goFlex

A terminal-based client for [FlexRadio](https://www.flexradio.com/) SmartSDR devices, written in Go. It provides an interactive TUI for discovering, connecting to, and interacting with FlexRadio transceivers over the network.

### 🚀 Features

*   **Interactive TUI** — Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for a rich terminal interface
*   **Radio Discovery** — Passive UDP discovery of FlexRadio devices on the local network
*   **Resilient Connections** — TCP keepalive, heartbeat pings, auto-reconnect with exponential back-off, and connection state tracking
*   **SmartSDR Protocol** — Full parser for the SmartSDR wire protocol (status updates, responses, handles)
*   **Configurable** — Environment-based configuration with `.env` file support
*   **Real-time Logging** — Live log view with word wrapping and scrollback

### ⚙️ Getting Started

#### Prerequisites

*   Go 1.26.2 or later

#### Installation

1.  **Clone the repository:**
    ```bash
    git clone <repository-url>
    cd goFlex
    ```

2.  **Install dependencies:**
    ```bash
    go mod tidy
    ```

3.  **Build (optional):**
    ```bash
    go build -o goFlex main.go
    ```

### 🚀 Usage

Run the application directly:

```bash
go run main.go
```

Or run the built binary:

```bash
./goFlex
```

The TUI will start in an alternate screen. It will first scan for radios on the network, then allow you to connect and interact.

### ⚙️ Configuration

Configuration is loaded from environment variables or an optional `.env` file in the working directory.

| Variable | Default | Description |
|----------|---------|-------------|
| `RADIO_ADDRESS` | `192.168.50.151` | IP address or hostname of the FlexRadio |
| `RADIO_PORT` | `4992` | TCP port for the SmartSDR protocol |
| `MAX_LOG` | `500` | Maximum number of log entries to retain in the TUI |

Example `.env` file:

```dotenv
RADIO_ADDRESS=192.168.1.100
RADIO_PORT=4992
MAX_LOG=1000
```

### 🧪 Testing

Run all tests:

```bash
go test ./...
```

Run tests with verbose output:

```bash
go test -v ./...
```

### 📁 Project Structure

```
goFlex/
├── main.go              # Entry point
├── app/
│   ├── app.go           # Application bootstrap
│   ├── tui.go           # Bubble Tea model and UI
│   └── wordwrap.go      # Log word-wrapping utility
├── config/
│   └── config.go        # Environment / .env configuration
├── radio/
│   ├── radio.go         # TCP connection management
│   ├── discovery.go     # UDP radio discovery
│   └── protocol.go      # SmartSDR wire protocol parser
└── go.mod
```

### 🤝 Contributing

1.  Fork the Project
2.  Create a Feature Branch (`git checkout -b feature/AmazingFeature`)
3.  Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4.  Push to the Branch (`git push origin feature/AmazingFeature`)
5.  Open a Pull Request

### 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
