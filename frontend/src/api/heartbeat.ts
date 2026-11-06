// The heartbeats' API

import { request } from './client';
import type { Heartbeat, HeartbeatList } from '../schema/heartbeat';

// listHeartbeats returns every heartbeat of the session, those which fire
// soonest first, a page at a time, as the server returns at most 100 at
// once, until they're all listed, or a page is empty
export async function listHeartbeats(session: string, signal?: AbortSignal): Promise<HeartbeatList> {
    const all: Heartbeat[] = [];
    for (;;) {
        const page = await request<HeartbeatList>('GET', '/heartbeat', { query: { session, offset: all.length }, signal });
        const body = page.body ?? [];
        all.push(...body);
        if (body.length === 0 || all.length >= page.count) {
            return { count: page.count, body: all };
        }
    }
}

// deleteHeartbeat deletes the heartbeat with the identifier, and returns it,
// as it was
export function deleteHeartbeat(id: string): Promise<Heartbeat> {
    return request<Heartbeat>('DELETE', '/heartbeat/' + encodeURIComponent(id));
}
