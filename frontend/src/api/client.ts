// A client for the server's JSON API, which is served beneath the prefix in
// the server's configuration

import type { Config } from '../schema/config';

// The configuration, which is read once
let configuration: Promise<Config> | undefined;

// Query parameter values. Arrays are repeated parameters, and undefined and
// null values are left out.
export type QueryValue = string | number | boolean | undefined | null | (string | number | boolean)[];

export interface RequestOptions {
    query?: Record<string, QueryValue>;
    body?: unknown;
    signal?: AbortSignal;
}

// ApiError is an error returned by the server, which mirrors
// httpresponse.ErrResponse in go-server
export class ApiError extends Error {
    readonly code: number;
    readonly detail?: unknown;

    constructor(code: number, reason: string, detail?: unknown) {
        super(reason);
        this.name = 'ApiError';
        this.code = code;
        this.detail = detail;
    }
}

// config returns the server's configuration, from config.json, which is
// beside the page, as it was when the page was loaded, which is the page's.
// It's read again when reading it failed.
export function config(): Promise<Config> {
    configuration ??= latestConfig().catch((error) => {
        configuration = undefined;
        throw error;
    });
    return configuration;
}

// latestConfig returns the server's configuration, as it is now, rather
// than as it was when the page was loaded, such as when the server was
// updated while it was down, from config.json, which isn't cached
export async function latestConfig(): Promise<Config> {
    const response = await fetch('config.json', { headers: { Accept: 'application/json' }, cache: 'no-store' });
    if (!response.ok) {
        throw new ApiError(response.status, `configuration: ${response.statusText}`);
    }
    return response.json() as Promise<Config>;
}

// request makes a request to the path, beneath the API's prefix, and returns
// the response, parsed as JSON. A body which is a blob, such as a file, is
// sent as it is, with its type, and anything else as JSON.
export async function request<T>(method: string, path: string, options: RequestOptions = {}): Promise<T> {
    const { prefix } = await config();
    const headers: Record<string, string> = { Accept: 'application/json' };
    const blob = options.body instanceof Blob ? options.body : undefined;
    if (blob) {
        headers['Content-Type'] = blob.type || 'application/octet-stream';
    } else if (options.body !== undefined) {
        headers['Content-Type'] = 'application/json';
    }

    const response = await fetch(prefix + path + queryString(options.query), {
        method,
        headers,
        body: blob ?? (options.body === undefined ? undefined : JSON.stringify(options.body)),
        signal: options.signal,
    });

    // The server returns errors as JSON, but a proxy may not
    if (!response.ok) {
        const error = await response.json().catch(() => undefined);
        throw new ApiError(response.status, error?.reason || response.statusText, error?.detail);
    }
    return response.json() as Promise<T>;
}

// StreamOptions are the options of a request for server-sent events, with
// the function which is called once the server has responded, before the
// first event
export interface StreamOptions extends RequestOptions {
    onOpen?: () => void;
}

// requestStream makes a request to the path, beneath the API's prefix, which
// accepts server-sent events, and calls the function with the name and data,
// parsed as JSON, of each event, as it's received, until the response ends
export async function requestStream(method: string, path: string, fn: (event: string, data: unknown) => void, options: StreamOptions = {}): Promise<void> {
    const { prefix } = await config();
    const headers: Record<string, string> = { Accept: 'text/event-stream' };
    if (options.body !== undefined) {
        headers['Content-Type'] = 'application/json';
    }
    const response = await fetch(prefix + path + queryString(options.query), {
        method,
        headers,
        body: options.body === undefined ? undefined : JSON.stringify(options.body),
        signal: options.signal,
    });
    if (!response.ok) {
        const error = await response.json().catch(() => undefined);
        throw new ApiError(response.status, error?.reason || response.statusText, error?.detail);
    } else if (!response.body) {
        throw new ApiError(response.status, 'missing response body');
    }
    options.onOpen?.();

    // Events are separated by blank lines, and each is lines of fields, of
    // which event is its name, and data its data, which is joined by newlines
    // when there's more than one
    const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
    let buffer = '';
    for (;;) {
        const { value, done } = await reader.read();
        if (done) {
            return;
        }
        buffer += value.replace(/\r\n?/g, '\n');
        let end: number;
        while ((end = buffer.indexOf('\n\n')) >= 0) {
            const block = buffer.slice(0, end);
            buffer = buffer.slice(end + 2);
            let event = 'message';
            const data: string[] = [];
            for (const line of block.split('\n')) {
                const colon = line.indexOf(':');
                const field = colon < 0 ? line : line.slice(0, colon);
                const text = colon < 0 ? '' : line.slice(colon + 1).replace(/^ /, '');
                if (field === 'event') {
                    event = text;
                } else if (field === 'data') {
                    data.push(text);
                }
            }
            if (data.length > 0) {
                fn(event, JSON.parse(data.join('\n')));
            }
        }
    }
}

// toDate returns the date of an RFC 3339 string, or undefined when there's
// no string, as Go omits zero times
export function toDate(value: unknown): Date | undefined {
    return typeof value === 'string' ? new Date(value) : undefined;
}

// queryString returns the query, including the "?", or an empty string when
// there's nothing in it
function queryString(query?: Record<string, QueryValue>): string {
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(query ?? {})) {
        for (const item of Array.isArray(value) ? value : [value]) {
            if (item !== undefined && item !== null) {
                params.append(key, String(item));
            }
        }
    }
    const str = params.toString();
    return str ? '?' + str : '';
}
