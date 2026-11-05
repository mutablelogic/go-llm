package httpclient

import (
	"context"
	"io"
	"net/http"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// yamlRequest is a request whose body is YAML, which is read as it is
type yamlRequest struct {
	io.Reader
	method string
	accept string
}

// yamlResponse is a response whose body is YAML, which is written as it is
type yamlResponse struct {
	w io.Writer
}

var _ client.Payload = (*yamlRequest)(nil)
var _ client.Unmarshaler = (*yamlResponse)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateAgent creates an agent with the definition, and returns it
func (c *Client) CreateAgent(ctx context.Context, meta schema.AgentMeta) (*schema.Agent, error) {
	req, err := client.NewJSONRequestEx(http.MethodPost, meta, types.ContentTypeJSON)
	if err != nil {
		return nil, err
	}
	var response schema.Agent
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("llm", "agent")); err != nil {
		return nil, err
	}
	return &response, nil
}

// WriteAgentYAML creates an agent with the definition which is read from the
// reader, which is YAML, such as a file of an agent's definition, which the
// server parses, and returns it
func (c *Client) WriteAgentYAML(ctx context.Context, r io.Reader) (*schema.Agent, error) {
	req := &yamlRequest{Reader: r, method: http.MethodPost, accept: types.ContentTypeJSON}
	var response schema.Agent
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("llm", "agent")); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetAgent returns the agent with the name
func (c *Client) GetAgent(ctx context.Context, name string) (*schema.Agent, error) {
	var response schema.Agent
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath("llm", "agent", name)); err != nil {
		return nil, err
	}
	return &response, nil
}

// ReadAgentYAML writes the definition of the agent with the name to the
// writer, as YAML, as a file of it is, which WriteAgentYAML can create again
func (c *Client) ReadAgentYAML(ctx context.Context, name string, w io.Writer) error {
	return c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeYAML), &yamlResponse{w: w}, client.OptPath("llm", "agent", name))
}

// ListAgents returns the agents, in the server's order, as the request
// selects, and how many there are
func (c *Client) ListAgents(ctx context.Context, req schema.AgentListRequest) (*schema.AgentList, error) {
	var response schema.AgentList
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath("llm", "agent"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return &response, nil
}

// UpdateAgent changes the definition of the agent with the name: what the
// update sets is changed, and the rest kept. Returns the agent.
func (c *Client) UpdateAgent(ctx context.Context, name string, meta schema.AgentMeta) (*schema.Agent, error) {
	req, err := client.NewJSONRequestEx(http.MethodPatch, meta, types.ContentTypeJSON)
	if err != nil {
		return nil, err
	}
	var response schema.Agent
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("llm", "agent", name)); err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteAgent deletes the agent with the name, and returns it, as it was
func (c *Client) DeleteAgent(ctx context.Context, name string) (*schema.Agent, error) {
	var response schema.Agent
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodDelete, types.ContentTypeJSON), &response, client.OptPath("llm", "agent", name)); err != nil {
		return nil, err
	}
	return &response, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (r *yamlRequest) Method() string {
	return r.method
}

func (r *yamlRequest) Accept() string {
	return r.accept
}

func (r *yamlRequest) Type() string {
	return types.ContentTypeYAML
}

// Unmarshal writes the body of the response to the writer, as it is
func (r *yamlResponse) Unmarshal(_ http.Header, body io.Reader) error {
	_, err := io.Copy(r.w, body)
	return err
}
