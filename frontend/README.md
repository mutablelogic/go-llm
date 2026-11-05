# Frontend

The web UI, which the server embeds and serves from its root, with the API under its prefix, such as `/api`: **Chat**, to chat in sessions, **Models**, the models of the providers, and **Tools**, the tools and agents which models can call. It's written in TypeScript, with [Lit](https://lit.dev/) and [Carbon's web components](https://web-components.carbondesignsystem.com/), and built with esbuild.

## Building

`make frontend` installs the dependencies from `package-lock.json`, checks the types with `tsc`, and builds the UI into `build/frontend`:

- `build/frontend/dist` is the UI: `index.html` and its icon, from `assets/dist`, and `index.js` and `index.css`, bundled from `src`, with the IBM Plex fonts.
- `build/frontend/embed.go` is copied from `assets/embed.go`. It's the `github.com/mutablelogic/go-llm/build/frontend` package, which embeds `dist`.

The server imports that package only when it's built with the `frontend` tag, which `make` does, once it's built the UI, so `go install`, and `go test`, don't need Node, and serve the API without the UI. `make tidy` builds the UI first, as `go mod tidy` looks at every tag.

## Serving

`frontend/cmd` registers the UI with the server's router, when it's built in, unless it's run with `--no-ui`, and `/config.json`, which the UI reads to find the API's prefix, the server's version, and the API's documentation, when it's served.

## Source

| Path | What it is |
|---|---|
| `assets/dist/index.html` | The page: the header, the side navigation, and the pages, of which one is shown |
| `src/index.ts` | Loads the components and pages, starts the controllers, watches the server's events, and starts the router |
| `src/router.ts` | Shows the page of the location's hash, such as `#chat/<id>`, and hides the others |
| `src/api/` | Requests the API, beneath the prefix in `config.json`, or streams its server-sent events, such as a reply, or the server's changes |
| `src/controller/events.ts` | Keeps watching the server's changes, and calls the listeners of the tables which change, such as `llm` `session`, which load what changed |
| `src/controller/notifications.ts` | Shows notifications as toasts, such as when the server can't be reached |
| `src/controller/list.ts` | `ListController`, which loads a page of a list, such as the models, and shows it in a table, and the item which is clicked in a panel |
| `src/controller/chat.ts` | The chats: the sessions, the conversation of the one which is open, which loads again when it changes, such as when another window replies, sending a question, and streaming its reply, saying someone is typing, and starting, and deleting, chats |
| `src/controller/models.ts`, `tools.ts` | The models, and the tools, in their tables, and the one which is clicked in a panel |
| `src/schema/` | The types of the API's JSON |
| `src/page/` | The pages, which extend `Page`, with a heading and a description |
| `src/component/` | The components which pages are made of: the session list, the chat view, which renders replies as Markdown, sanitized, the new chat panel, the model and tool tables, and their panels, as `Panel`'s are, and the side navigation's footer |

The elements are named `llm-…`.

## Adding a page

1. Add the page in `src/page`, extending `Page`, and import it in `src/index.ts`.
2. Add its element to `<main>` in `assets/dist/index.html`, hidden, and a link to it, such as `#models`, in the side navigation.
3. Add it to the router in `src/index.ts`, with the function which loads it when it's shown, if it has one.
4. When what it shows changes, add a listener to the events controller in `src/index.ts`, such as `events.on({ schema: 'llm', table: 'provider' }, …)`, which loads it again. The registry publishes changes to the `llm` schema's `provider` and `session` tables, the toolkit to its `tool` table, and the memory to the `memory` schema's `detail` and `memory` tables.
