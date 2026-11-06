// The events' API

import { requestStream } from './client';
import type { Event, EventFilter } from '../schema/event';

// The names of the server-sent events: each event, and the one which is sent
// when the server stops, before the stream ends
const eventName = 'event';
const eventShutdown = 'shutdown';

// watchEvents calls the function with each event which matches the filter,
// as it happens, until the stream ends or the signal aborts it, and returns
// true when the server said it's stopping, even when the stream then fails,
// as it can, as the server closes it. The open function is called once the
// stream has started, and the shutdown function as soon as the server says
// it's stopping. The stream's pings are ignored.
export async function watchEvents(fn: (event: Event) => void, filter: EventFilter = {}, signal?: AbortSignal, onOpen?: () => void, onShutdown?: () => void): Promise<boolean> {
    let shutdown = false;
    try {
        await requestStream('GET', '/events', (name, data) => {
            if (name === eventName) {
                fn(data as Event);
            } else if (name === eventShutdown && !shutdown) {
                shutdown = true;
                onShutdown?.();
            }
        }, { query: { ...filter }, signal, onOpen });
    } catch (error) {
        if (!shutdown) {
            throw error;
        }
    }
    return shutdown;
}
