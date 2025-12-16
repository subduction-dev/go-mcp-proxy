# go-mcp-proxy

A proxy for connecting MCP clients to remote MCP servers with OAuth authentication support.

## Features

- Proxies MCP (Model Context Protocol) requests between local clients and remote servers
- OAuth 2.0 authentication with PKCE support
- Dynamic client registration
- Automatic token storage and refresh

## Installation

```bash
brew tap subduction-dev/tap
brew install --cask subduction-dev/homebrew-tap/go-mcp-proxy
```

## Usage

```bash
go-mcp-proxy <server-url> [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--client-id` | | Optional OAuth Client ID |
| `--client-secret` | | Optional OAuth Client Secret |
| `--auth-port` | `8080` | Port to listen for authentication callbacks |
| `--scopes` | `openid,profile,email` | Scopes to request from the authorization server |
| `--data-path` | `~/.go-mcp-proxy` | Path to store tokens and client info |

## Claude Desktop Configuration

Add the following to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "continent-dev": {
      "command": "go-mcp-proxy",
      "args": [
        "https://whatever.com/mcp",
        "--scopes",
        "openid,profile,email,offline_access"
      ]
    }
  }
}
```

## How It Works

1. The proxy connects to the remote MCP server
2. If authentication is required, it opens a browser for OAuth login
3. After successful authentication, tokens are stored locally
4. The proxy forwards MCP tool calls between the client and server
