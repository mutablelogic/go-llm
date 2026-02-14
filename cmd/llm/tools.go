package main

import (
	"encoding/json"
	"fmt"

	// Packages
	homeassistant "github.com/mutablelogic/go-llm/pkg/homeassistant"
	newsapi "github.com/mutablelogic/go-llm/pkg/newsapi"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	weatherapi "github.com/mutablelogic/go-llm/pkg/weatherapi"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ToolCommands struct {
	ListTools ListToolsCommand `cmd:"" name:"tools" help:"List available tools." group:"TOOL"`
	ToolInfo  ToolInfoCommand  `cmd:"" name:"tool" help:"Show detailed information about a tool." group:"TOOL"`
	RunTool   RunToolCommand   `cmd:"" name:"run" help:"Run a tool with JSON input." group:"TOOL"`
}

type ListToolsCommand struct {
	JSON bool `name:"json" help:"Output as JSON"`
}

type ToolInfoCommand struct {
	Name string `arg:"" name:"name" help:"Tool name"`
	JSON bool   `name:"json" help:"Output as JSON"`
}

type RunToolCommand struct {
	Name  string          `arg:"" name:"name" help:"Tool name"`
	Input json.RawMessage `arg:"" name:"input" optional:"" help:"JSON input for the tool (optional)"`
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Toolkit returns a toolkit with all configured tools
func (g *Globals) Toolkit() (*tool.Toolkit, error) {
	var tools []tool.Tool

	// Add News API tools if key is set
	if g.NewsAPIKey != "" {
		newsTools, err := newsapi.NewTools(g.NewsAPIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create NewsAPI tools: %w", err)
		}
		tools = append(tools, newsTools...)
	}

	// Add Weather API tools if key is set
	if g.WeatherAPIKey != "" {
		weatherTools, err := weatherapi.NewTools(g.WeatherAPIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create WeatherAPI tools: %w", err)
		}
		tools = append(tools, weatherTools...)
	}

	// Add Home Assistant tools if endpoint and token are set
	if g.HAEndpoint != "" && g.HAToken != "" {
		haTools, err := homeassistant.NewTools(g.HAEndpoint, g.HAToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create Home Assistant tools: %w", err)
		}
		tools = append(tools, haTools...)
	}

	return tool.NewToolkit(tools...)
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *ListToolsCommand) Run(ctx *Globals) (err error) {
	toolkit, err := ctx.Toolkit()
	if err != nil {
		return err
	}

	tools := toolkit.Tools()
	if len(tools) == 0 {
		ctx.log.Print(ctx.ctx, "No tools available. Set NEWS_API_KEY and/or WEATHER_API_KEY to enable API tools.")
		return nil
	}

	if cmd.JSON {
		// Output as JSON
		type toolInfo struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		var output []toolInfo
		for _, tool := range tools {
			output = append(output, toolInfo{
				Name:        tool.Name(),
				Description: tool.Description(),
			})
		}
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		// Output as text
		for _, tool := range tools {
			ctx.log.Print(ctx.ctx, fmt.Sprintf("%-25s %s", tool.Name(), tool.Description()))
		}
	}

	return nil
}

func (cmd *ToolInfoCommand) Run(ctx *Globals) (err error) {
	toolkit, err := ctx.Toolkit()
	if err != nil {
		return err
	}

	// Lookup the tool
	tool := toolkit.Lookup(cmd.Name)
	if tool == nil {
		return fmt.Errorf("tool not found: %q", cmd.Name)
	}

	// Get the schema
	schema, err := tool.Schema()
	if err != nil {
		return fmt.Errorf("failed to get schema: %w", err)
	}

	if cmd.JSON {
		// Output as JSON
		type toolDetail struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Schema      any    `json:"schema,omitempty"`
		}
		output := toolDetail{
			Name:        tool.Name(),
			Description: tool.Description(),
			Schema:      schema,
		}
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		// Output as text
		fmt.Printf("Name: %s\n", tool.Name())
		fmt.Printf("Description: %s\n", tool.Description())
		if schema != nil {
			fmt.Println("\nSchema:")
			data, err := json.MarshalIndent(schema, "  ", "  ")
			if err != nil {
				return err
			}
			fmt.Printf("  %s\n", string(data))
		}
	}

	return nil
}

func (cmd *RunToolCommand) Run(ctx *Globals) (err error) {
	toolkit, err := ctx.Toolkit()
	if err != nil {
		return err
	}

	// Prepare input (nil if not provided)
	var input any
	if len(cmd.Input) > 0 {
		input = cmd.Input
	}

	// Run the tool using the toolkit (which handles JSON unmarshaling and validation)
	result, err := toolkit.Run(ctx.ctx, cmd.Name, input)
	if err != nil {
		return err
	}

	// Output the result as JSON
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}
	fmt.Println(string(data))

	return nil
}
