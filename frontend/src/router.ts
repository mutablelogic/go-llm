// The router, which shows the page of the location's hash, such as #models
// or #chat/<id>, and hides the others, as the location changes

// Route is a page, its parameters, and its query, from the location's hash,
// whose query says what's shown over the page, such as in a panel
export interface Route {
    readonly page: string;
    readonly params: readonly string[];
    readonly query: URLSearchParams;
}

// parseRoute returns the route of a location's hash, whose elements are
// separated by /, such as #chat/<id>, whose page is chat, and whose
// parameters are photos, or the default page when the hash is empty, with
// its query, after a ?, such as #models?provider=ollama
export function parseRoute(hash: string, defaultPage: string): Route {
    const [path, query = ''] = hash.replace(/^#/, '').split(/\?(.*)/s);
    const [page, ...params] = path.split('/').map((element) => {
        try {
            return decodeURIComponent(element);
        } catch {
            return element;
        }
    });
    return { page: page || defaultPage, params, query: new URLSearchParams(query) };
}

// routeHref returns the link to a page, with its parameters
export function routeHref(page: string, ...params: string[]): string {
    return '#' + [page, ...params].map(encodeURIComponent).join('/');
}

// withQuery returns the link, without its query, with the query, or without
// one, when it's empty
export function withQuery(href: string, query: Record<string, string> = {}): string {
    const path = href.split('?')[0];
    const str = new URLSearchParams(query).toString();
    return str ? `${path}?${str}` : path;
}

export class Router {
    // The pages, by their names, and the page shown for a route whose page
    // isn't one of them
    private readonly pages = new Map<string, HTMLElement>();
    private readonly notFound: HTMLElement;
    private readonly defaultPage: string;

    // The functions called when a page is shown, with its route, by the
    // page's name, and the functions called with every route
    private readonly enters = new Map<string, (route: Route) => void>();
    private readonly listeners: ((route: Route) => void)[] = [];

    constructor(notFound: HTMLElement, defaultPage: string) {
        this.notFound = notFound;
        this.defaultPage = defaultPage;
    }

    // add adds a page, which is shown for routes to its name, calling enter,
    // when it's set, with the route
    add(name: string, page: HTMLElement | null, enter?: (route: Route) => void): this {
        if (page) {
            this.pages.set(name, page);
            if (enter) {
                this.enters.set(name, enter);
            }
        }
        return this;
    }

    // listen calls the function with every route, such as to show which is
    // active in the navigation
    listen(listener: (route: Route) => void): this {
        this.listeners.push(listener);
        return this;
    }

    // start shows the page of the location, and the page of each location
    // the location changes to
    start() {
        window.addEventListener('hashchange', () => this.route());
        this.route();
    }

    // route shows the page of the location, and hides the others
    private route() {
        const route = parseRoute(location.hash, this.defaultPage);
        const page = this.pages.get(route.page);
        for (const other of [...this.pages.values(), this.notFound]) {
            other.hidden = other !== (page ?? this.notFound);
        }
        this.enters.get(route.page)?.(route);
        for (const listener of this.listeners) {
            listener(route);
        }
    }
}
