// The events' schema, which mirrors the broadcaster's schema

// Event is a change to the server's data: how it changed, which is create,
// update or delete, the schema and table which changed, such as llm and
// session, the row which changed, such as a session's identifier, and what
// the table says of the change, such as a session's serial
export interface Event {
    action?: string;
    schema?: string;
    table?: string;
    row?: string;
    data?: unknown;
}

// EventFilter matches events with its schema, table and action, each of which
// matches any event when it's undefined
export type EventFilter = Omit<Event, 'row' | 'data'>;

// eventMatches returns true when the event matches the filter
export function eventMatches(filter: EventFilter, event: Event): boolean {
    return (!filter.schema || filter.schema === event.schema) &&
        (!filter.table || filter.table === event.table) &&
        (!filter.action || filter.action === event.action);
}
