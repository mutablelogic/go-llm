// The heartbeats' API

import { request } from './client';
import type { Heartbeat, HeartbeatList } from '../schema/heartbeat';

// listHeartbeats returns the heartbeats of the session, those which fire
// soonest first
export function listHeartbeats(session: string, signal?: AbortSignal): Promise<HeartbeatList> {
    return request<HeartbeatList>('GET', '/heartbeat', { query: { session }, signal });
}

// deleteHeartbeat deletes the heartbeat with the identifier, and returns it,
// as it was
export function deleteHeartbeat(id: string): Promise<Heartbeat> {
    return request<Heartbeat>('DELETE', '/heartbeat/' + encodeURIComponent(id));
}
