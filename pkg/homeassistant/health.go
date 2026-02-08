package homeassistant

import "context"

///////////////////////////////////////////////////////////////////////////////
// API CALLS

// Health returns "API Running" if the Home Assistant API is reachable
func (c *Client) Health(ctx context.Context) (string, error) {
	// Response schema
	type responseHealth struct {
		Message string `json:"message"`
	}

	// Return the response
	var response responseHealth
	if err := c.DoWithContext(ctx, nil, &response); err != nil {
		return "", err
	}

	// Return success
	return response.Message, nil
}
