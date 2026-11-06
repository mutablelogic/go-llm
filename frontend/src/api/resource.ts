// The resources' API

import { config, request } from './client';
import type { Resource, ResourceList } from '../schema/resource';

// listResources returns a page of the session's resources, the most recently
// used first
export function listResources(session: string, offset: number, limit: number, signal?: AbortSignal): Promise<ResourceList> {
    return request<ResourceList>('GET', '/resource', { query: { session, offset, limit }, signal });
}

// createResource uploads the file as a resource of the session, with its name
// and type, and returns it, which is the one in the session with its content
// already, when there's one, so a file which is uploaded again isn't kept
// twice
export function createResource(session: string, file: File): Promise<Resource> {
    return request<Resource>('POST', '/resource', { query: { session, name: file.name || undefined, type: file.type || undefined }, body: file });
}

// deleteResource deletes the resource from the session
export function deleteResource(session: string, hash: string): Promise<Resource> {
    return request<Resource>('DELETE', '/resource/' + hash, { query: { session } });
}

// contentURL returns the URL of the content of the resource, in the session,
// or on its own, which is shown, or played, by the browser, or, with
// download, saved by it
export async function contentURL(hash: string, session?: string, download = false): Promise<string> {
    const { prefix } = await config();
    const query = new URLSearchParams();
    if (session) {
        query.set('session', session);
    }
    if (download) {
        query.set('download', 'true');
    }
    const str = query.toString();
    return `${prefix}/resource/${hash}/content${str ? '?' + str : ''}`;
}
