// Links to the pages, whose router is go-server's, which shows the page of
// the location's hash, such as #models or #chat/<id>

// routeHref returns the link to a page, with its parameters
export function routeHref(page: string, ...params: string[]): string {
    return '#' + [page, ...params].map(encodeURIComponent).join('/');
}
