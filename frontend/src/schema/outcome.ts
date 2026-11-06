// The outcomes of events, such as heartbeats which fired, which mirrors the
// registry's schema of an event

// EventOutcome is what an event which ran replied: the session it ran in, the
// session it's for, what it's for, its message, how important it is, what's
// become of it, and when it was due, and fired
export interface EventOutcome {
    id: string;
    serial?: number;
    run?: string;
    session?: string;
    title?: string;
    message?: string;
    importance?: 'ignore' | 'notify' | 'alert';
    status?: 'scheduled' | 'failed' | 'ignored' | 'pending' | 'notified' | 'delivered';
    source?: string;
    scheduled?: string;
    fired?: string;
    created?: string;
    modified?: string;
}

// EventOutcomeList is a page of outcomes, the most recent first, and how many
// there are
export interface EventOutcomeList {
    count: number;
    body?: EventOutcome[];
}
