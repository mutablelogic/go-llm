package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"os/user"
	"syscall"
	"time"

	// Packages
	"github.com/mutablelogic/go-llm/pkg/toolkit"
	"github.com/mutablelogic/go-llm/pkg/toolkit/resource"
	"golang.org/x/sync/errgroup"
)

func main() {
	// Create a toolkit with builtins and a handler for connector events and prompt execution.
	h := NewHandler()
	tk, err := toolkit.New(
		toolkit.WithHandler(h),
	)
	if err != nil {
		log.Fatal(err)
	}
	h.SetToolkit(tk)

	// Add a remote MCP connector — namespace inferred from the server.
	// Can be called before or while Run is active.
	if err = tk.AddConnector("https://remote.mcpservers.org/fetch/mcp"); err != nil {
		log.Fatal(err)
	}

	// Or provide an explicit namespace.
	if err = tk.AddConnectorNS("my-server", "https://remote.mcpservers.org/sequentialthinking/mcp"); err != nil {
		log.Fatal(err)
	}

	// Run until CTRL-C.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create an errgroup to manage concurrent goroutines.
	g, ctx := errgroup.WithContext(ctx)

	// Run starts all connectors and blocks until ctx is cancelled.
	// It closes the toolkit and waits for all connectors to finish on return.
	// Connectors can be added and removed while Run is active.
	g.Go(func() error {
		return tk.Run(ctx)
	})

	// Register prompts from the embedded agent filesystem.
	g.Go(func() error {
		prompts, err := CreatePrompts()
		if err != nil {
			return err
		}
		return tk.AddPrompt(prompts...)
	})

	// Register resources from the embedded testdata filesystem.
	g.Go(func() error {
		resources, err := CreateResources()
		if err != nil {
			return err
		}
		return tk.AddResource(resources...)
	})

	// Register builtin tools.
	g.Go(func() error {
		tools, err := CreateTools()
		if err != nil {
			return err
		}
		return tk.AddTool(tools...)
	})

	// After a short delay, call builtin.greet with a name.
	g.Go(func() error {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
		}

		// get the user
		user, err := user.Current()
		if err != nil {
			return err
		}

		// Call the greet tool with the user's name.
		input, err := resource.JSON("input", greetRequest{Name: user.Name})
		if err != nil {
			return err
		}

		// Call the tool by name, which will route to the local implementation since it's registered as "builtin.greet".
		result, err := tk.Call(ctx, "builtin.greet", input)
		if err != nil {
			slog.Error("greet", "err", err)
			return nil
		}

		// Log the result if there is one.
		if result != nil {
			data, err := result.Read(ctx)
			if err != nil {
				slog.Error("greet", "err", err)
			} else {
				slog.Info("greet", "result", data)
			}
		}
		return nil
	})

	// After a short delay, call fetch to get a webpage
	g.Go(func() error {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(3 * time.Second):
		}

		// fetchRequest matches the input schema of the mcp-fetch.fetch tool.
		type fetchRequest struct {
			URL        string `json:"url"`
			MaxLength  int    `json:"max_length,omitempty"`
			StartIndex int    `json:"start_index,omitempty"`
			Raw        bool   `json:"raw,omitempty"`
		}

		// Call the tool without a namespace, which will route to the remote implementation
		input, err := resource.JSON("input", fetchRequest{URL: "https://news.bbc.co.uk/", MaxLength: 100})
		if err != nil {
			slog.Error("fetch", "err", err)
			return nil
		}
		result, err := tk.Call(ctx, "fetch", input)
		if err != nil {
			slog.Error("fetch", "err", err)
			return nil
		}

		// Log the result if there is one.
		if result != nil {
			data, err := result.Read(ctx)
			if err != nil {
				slog.Error("fetch", "err", err)
			} else {
				slog.Info("fetch", "result", data)
			}
		}
		return nil
	})

	// Wait for cancellation and log any errors.
	if err = g.Wait(); err != nil {
		if errors.Is(err, context.Canceled) {
			slog.Info("toolkit stopped gracefully")
		} else {
			log.Fatal(err)
		}
	}
}
