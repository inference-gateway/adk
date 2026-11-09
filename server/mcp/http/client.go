package http

import (
	"context"
	"strings"

	golang "github.com/metoro-io/mcp-golang"
	http "github.com/metoro-io/mcp-golang/transport/http"

	server "github.com/inference-gateway/adk/server"
)

// httpMCPClient adapts metoro-io/mcp-golang's streamable HTTP client to
// server.MCPToolCaller.
type httpMCPClient struct {
	url   string
	inner *golang.Client
}

// NewHTTPMCPClient builds an MCP client for a server base URL and endpoint
// path over streamable HTTP. Wire it with server.WithMCPClientFactory.
func NewHTTPMCPClient(url, endpoint string) server.MCPToolCaller {
	transport := http.NewHTTPClientTransport(endpoint)
	transport.WithBaseURL(url)
	return &httpMCPClient{
		url:   url,
		inner: golang.NewClient(transport),
	}
}

// Initialize performs the MCP handshake with the server.
func (c *httpMCPClient) Initialize(ctx context.Context) error {
	_, err := c.inner.Initialize(ctx)
	return err
}

// ListTools returns the server's tool metadata mapped onto server.MCPToolEntry.
func (c *httpMCPClient) ListTools(ctx context.Context) ([]server.MCPToolEntry, error) {
	// ponytail: single page; add cursor paging if a server ever returns NextCursor.
	resp, err := c.inner.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	entries := make([]server.MCPToolEntry, 0, len(resp.Tools))
	for _, t := range resp.Tools {
		description := ""
		if t.Description != nil {
			description = *t.Description
		}
		entries = append(entries, server.MCPToolEntry{
			Server:      c.url,
			Name:        t.Name,
			Description: description,
			InputSchema: t.InputSchema,
		})
	}
	return entries, nil
}

// CallTool invokes a tool by name and returns its joined text content.
func (c *httpMCPClient) CallTool(ctx context.Context, name string, arguments map[string]any) (string, error) {
	resp, err := c.inner.CallTool(ctx, name, arguments)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(resp.Content))
	for _, content := range resp.Content {
		if content != nil && content.TextContent != nil {
			parts = append(parts, content.TextContent.Text)
		}
	}
	return strings.Join(parts, ""), nil
}
