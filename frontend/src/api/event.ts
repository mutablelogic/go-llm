// The events' API

import { requestStream } from './client';
import type { Event, EventFilter } from '../schema/event';

// The names of the server-sent events: each event, and the one which is sent
// when the server stops, before the stream ends
const eventName = 'event';
const eventShutdown = 'shutdown';

// watchEvents calls the function with each event which matches the filter,
// as it happens, until the stream ends or the signal aborts it, and returns
// true when the stream ended as the server is stopping. The open function is
// called once the stream has started. The stream's pings are ignored.
export async function watchEvents(fn: (event: Event) => void, filter: EventFilter = {}, signal?: AbortSignal, onOpen?: () => void): Promise<boolean> {
    let shutdown = false;
    await requestStream('GET', '/llm/event', (name, data) => {
        if (name === eventName) {
            fn(data as Event);
        } else if (name === eventShutdown) {
            shutdown = true;
        }
    }, { query: { ...filter }, signal, onOpen });
    return shutdown;
}
