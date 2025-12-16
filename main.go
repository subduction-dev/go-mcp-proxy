package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

var MCPProxyCmd = &cobra.Command{
	Use:   "mcp-proxy",
	Short: "MCP Proxy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.SetOutput(os.Stderr)

		config.URL = args[0]
		p := NewProxy(config)

		return p.run(cmd.Context())
	},
}

var config = &Config{}

func main() {
	MCPProxyCmd.Flags().StringVar(&config.ClientID, "client-id", "", "Optional Client ID")
	MCPProxyCmd.Flags().StringVar(&config.ClientSecret, "client-secret", "", "Optional Client Secret")
	MCPProxyCmd.Flags().UintVar(&config.AuthServerPort, "auth-port", 8080, "Port to listen for authentication requests on")
	MCPProxyCmd.Flags().StringSliceVar(&config.Scopes, "scopes", []string{"openid", "profile", "email"}, "Scopes to request from the authorization server")
	MCPProxyCmd.Flags().StringVar(&config.StorageRoot, "data-path", "~/.go-mcp-proxy", "Path to store data")

	err := MCPProxyCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}
