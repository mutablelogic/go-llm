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

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/pkg/toolkit"
	resource "github.com/mutablelogic/go-llm/pkg/toolkit/resource"
	errgroup "golang.org/x/sync/errgroup"
)

func main() {
	// Create a toolkit with builtins and a handler for connector events and prompt execution.
	d := NewDelegate()
	tk, err := toolkit.New(
		toolkit.WithDelegate(d),
	)
	if err != nil {
		log.Fatal(err)
	}
	d.SetToolkit(tk)

	// Add a remote MCP connector — namespace inferred from the server.
	// Can be called before or while Run is active.
	if err = tk.AddConnector("https://remote.mcpservers.org/fetch/mcp"); err != nil {
		log.Fatal(err)
	}

	// Or provide an explicit namespace.
	if err = tk.AddConnectorNS("my-server", "https://remote.mcpservers.org/sequentialthinking/mcp"); err != nil {
		log.Fatal(err)
	}

	// Register prompts, resources, and builtin tools synchronously — all three
	// methods are safe to call before Run and complete before any Call is made.
	prompts, err := CreatePrompts()
	if err != nil {
		log.Fatal(err)
	}
	if err = tk.AddPrompt(prompts...); err != nil {
		log.Fatal(err)
	}

	resources, err := CreateResources()
	if err != nil {
		log.Fatal(err)
	}
	if err = tk.AddResource(resources...); err != nil {
		log.Fatal(err)
	}

	tools, err := CreateTools()
	if err != nil {
		log.Fatal(err)
	}
	if err = tk.AddTool(tools...); err != nil {
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

	// Call builtin.greet now — tools are already registered synchronously above.
	g.Go(func() error {
		select {
		case <-ctx.Done():
			return nil
		default:
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

	// Call fetch once the remote connector that serves it is ready.
	// Retry on ErrNotFound, waiting for the next tool-list refresh each time,
	// so the call succeeds regardless of which connector connects first.
	g.Go(func() error {
		// fetchRequest matches the input schema of the mcp-fetch.fetch tool.
		type fetchRequest struct {
			URL        string `json:"url"`
			MaxLength  int    `json:"max_length,omitempty"`
			StartIndex int    `json:"start_index,omitempty"`
			Raw        bool   `json:"raw,omitempty"`
		}
		input, err := resource.JSON("input", fetchRequest{URL: "https://news.bbc.co.uk/", MaxLength: 100})
		if err != nil {
			return err
		}

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-d.ToolsChanged():
			}

			result, err := tk.Call(ctx, "fetch", input)
			if errors.Is(err, llm.ErrNotFound) {
				// Tool not yet registered; wait for the next refresh.
				continue
			}
			if err != nil {
				slog.Error("fetch", "err", err)
				return nil
			}
			if result != nil {
				data, err := result.Read(ctx)
				if err != nil {
					slog.Error("fetch", "err", err)
				} else {
					slog.Info("fetch", "result", data)
				}
			}
			return nil
		}
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
