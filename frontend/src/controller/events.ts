// The events controller, which watches the server's events, and calls the
// listeners whose filter each event matches, so they load what changed. Every
// listener is called once the stream has started, which is once the server
// has subscribed to the events, as changes may have been made before then,
// when it's first watched, or while it wasn't. When the server stops, or the
// stream fails, the disconnect functions are called, as what's shown may no
// longer be so, and it's watched again, after a delay which grows while it
// fails, until it starts. A stream which ends otherwise, such as at the
// server's timeout, is watched again after the shortest delay. A listener is
// called at most once in an interval, after the first event in it, with the
// last, as a burst of events, such as as a reply is stored, is one change
// to load. When it fails, a notification says why, once until it starts, or,
// when the server is stopping, says so, and it's watched again, quietly, until
// the server is back.

import { watchEvents } from '../api/event';
import { eventMatches, type Event, type EventFilter } from '../schema/event';
import type { NotificationController } from './notifications';

// EventListener is called with an event which matches its filter, or with
// no event when events may have been missed
export type EventListener = (event?: Event) => void;

export class EventsController {
    // The delay before watching again, which doubles while it fails, up to
    // the most it can be
    static readonly minDelay = 1000;
    static readonly maxDelay = 30000;

    // The interval in which a listener is called at most once
    static readonly interval = 500;

    private readonly notifications: NotificationController;
    private readonly listeners: Listener[] = [];
    private readonly disconnects: (() => void)[] = [];
    private readonly shutdowns: (() => void)[] = [];
    private readonly connects: (() => void)[] = [];

    // The stream which is watched, which is aborted when it's stopped
    private request?: AbortController;

    constructor(notifications: NotificationController) {
        this.notifications = notifications;
    }

    // on calls the function with each event which matches the filter, or any
    // of the filters, or with no event when events may have been missed. The
    // function is called once for a burst of events which match any of them,
    // such as when a change is to more than one table, which a listener for
    // each would be called for.
    on(filter: EventFilter | EventFilter[], fn: EventListener): this {
        this.listeners.push(new Listener([filter].flat(), fn));
        return this;
    }

    // onDisconnect calls the function when the server stops, or the stream
    // fails, as the server can't be reached until it's watched again
    onDisconnect(fn: () => void): this {
        this.disconnects.push(fn);
        return this;
    }

    // onShutdown calls the function when the server says it's stopping, as
    // it's told before the stream ends, rather than when the stream fails,
    // so requests which would hold it up, such as a reply which is streamed,
    // can be stopped
    onShutdown(fn: () => void): this {
        this.shutdowns.push(fn);
        return this;
    }

    // onConnect calls the function whenever the stream starts, as the server
    // can be reached again, before the listeners are called
    onConnect(fn: () => void): this {
        this.connects.push(fn);
        return this;
    }

    // start watches the events, until it's stopped
    start() {
        this.stop();
        const request = new AbortController();
        this.request = request;
        void this.watch(request.signal);
    }

    // stop stops watching the events
    stop() {
        this.request?.abort();
        this.request = undefined;
    }

    // watch watches the events, and watches them again when the stream ends,
    // until the signal aborts it
    private async watch(signal: AbortSignal) {
        let delay = EventsController.minDelay;
        let failed = false;

        // Whether the disconnect functions have been called since the stream
        // last started, so they're called once, when the server stops, or the
        // stream fails, whether or not it had started
        let disconnected = false;
        const disconnect = () => {
            if (!disconnected) {
                disconnected = true;
                this.disconnects.forEach((fn) => fn());
            }
        };

        while (!signal.aborted) {
            // Once the stream has started, every listener is called, as
            // changes may have been made before the server subscribed, and
            // it's no longer failing
            const open = () => {
                delay = EventsController.minDelay;
                failed = false;
                disconnected = false;
                this.connects.forEach((fn) => fn());
                this.listeners.forEach((listener) => listener.call());
            };
            try {
                const shutdown = await watchEvents((event) => this.dispatch(event), {}, signal, open, () => this.shutdowns.forEach((fn) => fn()));
                if (signal.aborted) {
                    return;
                }
                if (shutdown) {
                    disconnect();
                    this.notifications.push({ kind: 'info', title: 'The server is stopping', subtitle: 'Changes are shown again once it is back' });
                    failed = true;
                }
            } catch (error) {
                if (signal.aborted) {
                    return;
                }
                disconnect();
                if (!failed) {
                    this.notifications.error("Changes couldn't be watched", error);
                    failed = true;
                }
                delay = Math.min(delay * 2, EventsController.maxDelay);
            }
            await new Promise((resolve) => setTimeout(resolve, delay));
        }
    }

    // dispatch calls the listeners whose filter the event matches
    private dispatch(event: Event) {
        for (const listener of this.listeners) {
            if (listener.filters.some((filter) => eventMatches(filter, event))) {
                listener.call(event);
            }
        }
    }
}

// Listener calls its function at most once in the interval, after the first
// call in it, with the last event, or no event when any call had none
class Listener {
    readonly filters: EventFilter[];
    private readonly fn: EventListener;
    private timer?: ReturnType<typeof setTimeout>;
    private event?: Event;
    private missed = false;

    constructor(filters: EventFilter[], fn: EventListener) {
        this.filters = filters;
        this.fn = fn;
    }

    call(event?: Event) {
        this.event = event;
        this.missed ||= event === undefined;
        this.timer ??= setTimeout(() => {
            const event = this.missed ? undefined : this.event;
            this.timer = undefined;
            this.event = undefined;
            this.missed = false;
            this.fn(event);
        }, EventsController.interval);
    }
}
