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
	"github.com/google/uuid"
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

	session := uuid.New().String()

	// Call greet and fetch concurrently
	g.Go(func() error {
		time.Sleep(2 * time.Second)
		u, err := user.Current()
		if err != nil {
			slog.Error("greet", "err", err)
			return nil
		}
		input, err := resource.JSON("input", greetRequest{Name: u.Name})
		if err != nil {
			slog.Error("greet", "err", err)
			return nil
		}
		if result, err := tk.Call(toolkit.WithSession(ctx, session), "builtin.greet", input); err != nil {
			slog.Error("greet", "err", err)
		} else if result != nil {
			if data, err := result.Read(ctx); err != nil {
				slog.Error("greet", "err", err)
			} else {
				slog.Info("greet", "result", data)
			}
		}
		return nil
	})

	g.Go(func() error {
		time.Sleep(2 * time.Second)
		input, err := resource.JSON("input", fetchRequest{URL: "https://news.bbc.co.uk/", MaxLength: 100})
		if err != nil {
			slog.Error("fetch", "err", err)
			return nil
		}
		if result, err := tk.Call(toolkit.WithSession(ctx, session), "fetch", input); err != nil {
			slog.Error("fetch", "err", err)
		} else if result != nil {
			if data, err := result.Read(ctx); err != nil {
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
