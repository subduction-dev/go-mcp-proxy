package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"runtime"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pkg/errors"
)

type Proxy struct {
	cfg         *Config
	store       *Storage
	oauthConfig client.OAuthConfig
	redirectUrl string
}

func NewProxy(cfg *Config) *Proxy {
	theUrl, err := url.Parse(cfg.URL)
	if err != nil {
		log.Fatalf("Failed to parse URL: %v", err)
	}

	store := NewStorage(path.Join(cfg.StorageRoot, theUrl.Hostname()+".json"))

	if cfg.ClientID == "" {
		clientInfo, err := store.GetClientInfo()
		if err != nil {
			log.Fatalf("Failed to get client info: %v", err)
		}
		if clientInfo != nil {
			cfg.ClientID = clientInfo.ID
			cfg.ClientSecret = clientInfo.Secret
		}
	}

	redirectUrl := fmt.Sprintf("http://localhost:%d/oauth/callback", cfg.AuthServerPort)
	oauthConfig := client.OAuthConfig{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		//AuthServerMetadataURL: fmt.Sprintf("https://%s/.well-known/oauth-authorization-server", cfg.WorkOS.Domain),
		RedirectURI: redirectUrl,
		Scopes:      cfg.Scopes,
		TokenStore:  store,
		PKCEEnabled: true,
	}

	return &Proxy{
		cfg:         cfg,
		store:       store,
		oauthConfig: oauthConfig,
		redirectUrl: redirectUrl,
	}
}

func (p *Proxy) startCallbackServer(callbackChan chan<- map[string]string) *http.Server {
	s := &http.Server{
		Addr: fmt.Sprintf(":%v", p.cfg.AuthServerPort),
	}

	http.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		// Extract query parameters
		params := make(map[string]string)
		for key, values := range r.URL.Query() {
			if len(values) > 0 {
				params[key] = values[0]
			}
		}

		// Send parameters to the channel
		callbackChan <- params

		// Respond to the user
		w.Header().Set("Content-Type", "text/html")
		_, err := w.Write([]byte(`
			<html>
				<body>
					<h1>Authorization Successful</h1>
					<p>You can now close this window and return to the application.</p>
					<script>window.close();</script>
				</body>
			</html>
		`))
		if err != nil {
			log.Printf("Error writing response: %v", err)
		}
	})

	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return s
}

func (p *Proxy) maybeAuthorize(err error) {
	// Check if we need OAuth authorization
	if client.IsOAuthAuthorizationRequiredError(err) {
		log.Println("OAuth authorization required. Starting authorization flow...")

		// Get the OAuth handler from the error
		oauthHandler := client.GetOAuthHandler(err)

		// Start a local server to handle the OAuth callback
		callbackChan := make(chan map[string]string)
		s := p.startCallbackServer(callbackChan)
		defer s.Close()

		// Generate PKCE code verifier and challenge
		codeVerifier, err := client.GenerateCodeVerifier()
		if err != nil {
			log.Fatalf("Failed to generate code verifier: %v", err)
		}
		codeChallenge := client.GenerateCodeChallenge(codeVerifier)

		// Generate state parameter
		state, err := client.GenerateState()
		if err != nil {
			log.Fatalf("Failed to generate state: %v", err)
		}

		if oauthHandler.GetClientID() == "" {
			log.Println("Registering client...")
			err = oauthHandler.RegisterClient(context.Background(), "go-mcp-proxy")
			if err != nil {
				log.Fatalf("Failed to register client: %v", err)
			}

			err = p.store.SaveClientInfo(&ClientInfo{
				ID:     oauthHandler.GetClientID(),
				Secret: oauthHandler.GetClientSecret(),
			})
			if err != nil {
				log.Fatalf("Failed to save client info: %v", err)
			}
		}

		// Get the authorization URL
		authURL, err := oauthHandler.GetAuthorizationURL(context.Background(), state, codeChallenge)
		if err != nil {
			log.Fatalf("Failed to get authorization URL: %v", err)
		}

		// Open the browser to the authorization URL
		log.Printf("Opening browser to: %s\n", authURL)
		p.openBrowser(authURL)

		// Wait for the callback
		log.Println("Waiting for authorization callback...")
		params := <-callbackChan

		//// Verify state parameter
		if params["state"] != state {
			log.Fatalf("State mismatch: expected %s, got %s", state, params["state"])
		}

		// Exchange the authorization code for a token
		code := params["code"]
		if code == "" {
			log.Fatalf("No authorization code received")
		}

		log.Println("Exchanging authorization code for token...")
		err = oauthHandler.ProcessAuthorizationResponse(context.Background(), code, state, codeVerifier)
		if err != nil {
			log.Fatalf("Failed to process authorization response: %v", err)
		}

		log.Println("Authorization successful!")
	}
}

func (p *Proxy) openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		log.Printf("Failed to open browser: %v", err)
		log.Printf("Please open the following URL in your browser: %s\n", url)
	}
}

func (p *Proxy) run(ctx context.Context) error {
	insecureTransport := http.DefaultTransport.(*http.Transport).Clone()
	insecureTransport.TLSClientConfig.InsecureSkipVerify = true

	c, err := client.NewOAuthStreamableHttpClient(
		p.cfg.URL,
		p.oauthConfig,
		transport.WithHTTPBasicClient(&http.Client{Transport: insecureTransport}),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create client")
	}
	defer c.Close()

	err = c.Start(ctx)
	if err != nil {
		p.maybeAuthorize(err)
		err = c.Start(ctx)
	}
	if err != nil {
		return errors.Wrap(err, "failed to start client")
	}

	initRequest := mcp.InitializeRequest{
		Params: struct {
			ProtocolVersion string                 `json:"protocolVersion"`
			Capabilities    mcp.ClientCapabilities `json:"capabilities"`
			ClientInfo      mcp.Implementation     `json:"clientInfo"`
		}{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "go-mcp-proxy",
				Version: "0.1.0",
			},
		},
	}
	initResponse, err := c.Initialize(ctx, initRequest)
	if err != nil {
		p.maybeAuthorize(err)
		initResponse, err = c.Initialize(ctx, initRequest)
	}
	if err != nil {
		return errors.Wrap(err, "failed to initialize client")
	}

	listToolsRequest := mcp.ListToolsRequest{}
	listToolsResponse, err := c.ListTools(ctx, listToolsRequest)
	if err != nil {
		p.maybeAuthorize(err)
		listToolsResponse, err = c.ListTools(ctx, listToolsRequest)
	}
	if err != nil {
		return errors.Wrap(err, "failed to list tools")
	}

	serverTools := make([]server.ServerTool, len(listToolsResponse.Tools))
	for i, tool := range listToolsResponse.Tools {
		serverTools[i] = server.ServerTool{
			Tool: tool,
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				result, err := c.CallTool(ctx, request)
				if err != nil {
					p.maybeAuthorize(err)
					result, err = c.CallTool(ctx, request)
				}
				return result, err
			},
		}
	}

	mcpServer := server.NewMCPServer(
		initResponse.ServerInfo.Name,
		initResponse.ServerInfo.Version,
		server.WithToolCapabilities(true),
		server.WithInstructions(initResponse.Instructions),
	)
	mcpServer.AddTools(serverTools...)

	return server.ServeStdio(mcpServer, server.WithErrorLogger(log.Default()))
}
