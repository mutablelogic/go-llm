// The heartbeats' schema, which mirrors the heartbeat package's

// Heartbeat is a prompt for a session, which fires once, at a time, or again
// and again, on a schedule, in its time zone, or the person's, with when it
// last fired, and when it fires next, unless it won't, as it fired once
export interface Heartbeat {
    id: string;
    title: string;
    prompt: string;
    session: string;
    schedule?: string;
    at?: string;
    timezone?: string;
    created?: string;
    modified?: string;
    fired?: string;
    next?: string;
}

// HeartbeatList is the heartbeats, those which fire soonest first, and how
// many there are
export interface HeartbeatList {
    count: number;
    body?: Heartbeat[];
}
