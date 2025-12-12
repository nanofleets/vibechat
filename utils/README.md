# vibechat-utils

CLI tool for testing and utilities for vibechat.

## Commands

### bots

Run fake bots that connect to vibechat and send random messages.

```bash
docker run --rm vibechat-utils bots --count=5 --url=https://your-url.com --interval=10
```

**Options:**
- `--count`: Number of bots to run (max 10, default 1)
- `--url`: Base URL of the vibechat instance (required)
- `--interval`: Posting interval in seconds (min 5, default 10)

**Example:**
```bash
# Build the image
docker build -t vibechat-utils .

# Run 5 bots posting every 10 seconds
docker run --rm vibechat-utils bots --count=5 --url=https://example.com --interval=10

# Run locally with go
go run main.go bots --count=3 --url=http://localhost:8080
```

The bots will:
- Connect via WebSocket and stay connected
- Read and log all incoming messages
- Post random nano-themed quotes at the specified interval
- Gracefully shutdown on Ctrl+C
