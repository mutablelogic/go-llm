# Frontend

The web UI, which the server embeds and serves from its root, with the API under its prefix, such as `/api`: **Conversations**, to chat in sessions, **Models**, the models of the providers, and **Tools**, the tools and agents which models can call. It's written in TypeScript, with [Lit](https://lit.dev/) and [Carbon's web components](https://web-components.carbondesignsystem.com/), on [go-server's frontend package](https://github.com/mutablelogic/go-server/tree/main/frontend), and built with esbuild.

## Building

`make frontend` installs the dependencies from `package-lock.json`, checks the types with `tsc`, and builds the UI into `build/frontend`. `@mutablelogic/go-server` is on GitHub Packages, so it needs `GITHUB_TOKEN`, with `read:packages`, such as `GITHUB_TOKEN=$(gh auth token) make`. `make docker` passes it to the build as a secret, which isn't kept in the image.

- `build/frontend/dist` is the UI: `index.html` and its icon, from `assets/dist`, and `index.js` and `index.css`, bundled from `src`, with the IBM Plex fonts.
- `build/frontend/embed.go` is copied from `assets/embed.go`. It's the `github.com/mutablelogic/go-llm/build/frontend` package, which embeds `dist`.

The server imports that package only when it's built with the `frontend` tag, which `make` does, once it's built the UI, so `go install`, and `go test`, don't need Node, and serve the API without the UI. `make tidy` builds the UI first, as `go mod tidy` looks at every tag.

## Serving

`frontend/cmd` registers the UI with the server's router, when it's built in, unless it's run with `--no-ui`, and `/config.json`, which the UI reads to find the API's prefix, the server's version, and the API's documentation, when it's served.

## Source

| Path | What it is |
|---|---|
| `assets/dist/index.html` | The page: go-server's header, and sidebar, with the media player and the server's status, the modules' pages, in `<main>`, and the notifications |
| `src/index.ts` | Reads the configuration, makes go-server's navigation, router, changes and notifications controllers, adds the modules, and starts the router and the changes |
| `src/module/` | The modules, Conversations, Models and Tools, each of which adds its page, its link in the sidebar, its route, and its controllers, and follows the changes which it shows |
| `src/router.ts` | `routeHref`, the link to a page, such as `#chat/<id>` |
| `src/api/` | Requests the API, beneath the prefix in `config.json`, or streams its server-sent events, such as a reply |
| `src/controller/list.ts` | `ListController`, which loads a page of a list, such as the models, and shows it in a table, and the item which is clicked in a panel |
| `src/controller/chat.ts` | The chats: the sessions, the conversation of the one which is open, which loads again when it changes, such as when another window replies, sending a question, and streaming its reply, saying someone is typing, and starting, and deleting, chats |
| `src/controller/models.ts`, `tools.ts` | The models, and the tools, in their tables, and the one which is clicked in a panel |
| `src/schema/` | The types of the API's JSON |
| `src/page/` | The pages, which extend `Page`, with a heading and a description |
| `src/component/` | The components which pages are made of: the session list, the chat view, which renders replies as Markdown, sanitized, the new chat panel, the model and tool tables, and their panels, as `Panel`'s are, and the media player |

The elements are named `llm-…`, and go-server's `gs-…`.

## Adding a module

1. Add its page in `src/page`, extending `Page`, and import it in `src/index.ts`.
2. Add the module in `src/module`, as a function of the `App`, which adds its page with `addPage`, its link to the sidebar with `app.nav.setModule(name, Section, pages)`, and its route with `app.router.add`.
3. When what it shows changes, it follows the changes with `app.changes.on({ schema, table }, …)`, which loads it again. The tables are in [Changes](../docs/changes.md).
4. Add it in `src/index.ts`.
