// The outcomes of events' API

import { request } from './client';
import type { EventOutcomeList } from '../schema/outcome';

// listEventOutcomes returns the outcomes of the events for the session, the
// most recent first, at most the limit of them
export function listEventOutcomes(session: string, limit: number, signal?: AbortSignal): Promise<EventOutcomeList> {
    return request<EventOutcomeList>('GET', '/llm/event', { query: { session, limit }, signal });
}
