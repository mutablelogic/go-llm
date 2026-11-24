//go:build !client

package main

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
	webhookcmd "github.com/mutablelogic/go-llm/webhook/cmd"
	registry "github.com/mutablelogic/go-llm/webhook/registry"
	server "github.com/mutablelogic/go-server"
	broadcaster "github.com/mutablelogic/go-server/broadcaster"
	broadcastercmd "github.com/mutablelogic/go-server/broadcaster/cmd"
	broadcasterhandler "github.com/mutablelogic/go-server/broadcaster/httphandler"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	errgroup "golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CLI struct {
	Server      ServerCommands                `embed:""`
	Webhook     webhookcmd.ClientCommands     `embed:""`
	Broadcaster broadcastercmd.ClientCommands `embed:""`
}

type ServerCommands struct {
	RunServer RunServer `cmd:"" name:"run" help:"Run the server." group:"SERVER"`
	servercmd.OpenAPICommands
}

type RunServer struct {
	Broadcaster broadcastercmd.Flags `embed:"" prefix:"webhook."`
	Webhook     webhookcmd.Flags     `embed:"" prefix:"webhook."`
	servercmd.RunServer
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run runs the server: the webhooks, in the store of its backend, which
// publishes their changes to its broadcaster, their API, and the URL which
// services call, under the server's prefix, and logs their calls, until
// it's stopped
func (runner *RunServer) Run(ctx server.Cmd) error {
	ctx.Logger().InfoContext(ctx.Context(), "starting server", "name", ctx.Name(), "version", ctx.Version(), "backend", Backend)
	return runner.Broadcaster.WithBroadcaster(ctx, &runner.RunServer, func(b *broadcaster.Broadcaster) error {
		// The broadcaster's changes are in the "LLM" group, with the
		// webhooks', as Redoc doesn't show a tag outside a group
		runner.RunServer.Register(func(router *httprouter.Router) error {
			router.Spec().AddTagGroup(llm.TagGroup, broadcasterhandler.Tag)
			return nil
		})

		// The store, of the backend, which publishes its changes
		store, err := newStore(ctx, b)
		if err != nil {
			return err
		}
		defer store.Close()

		return runner.Webhook.WithWebhook(ctx, &runner.RunServer, store, b, func(r *registry.Registry) error {
			// They all stop when any does, such as when the server is
			// stopped
			group, runCtx := errgroup.WithContext(ctx.Context())
			group.Go(func() error {
				return b.Run(runCtx)
			})
			group.Go(func() error {
				return r.Run(runCtx, ctx.Logger())
			})
			group.Go(func() error {
				return runner.RunServer.Run(ctx.WithContext(runCtx))
			})
			return group.Wait()
		})
	})
}
