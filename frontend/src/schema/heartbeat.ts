// The heartbeats' schema, which mirrors the heartbeat package's schema

// FireEvent is the data of the event of a heartbeat which fires: what it's
// for, its prompt, the session it's for, when it was due, and when it fired,
// which is later, when it's late
export interface FireEvent {
    heartbeat?: string;
    title?: string;
    prompt?: string;
    session?: string;
    scheduled?: string;
    fired?: string;
}
