// The workspaces' API

import { config, request } from './client';
import type { ResourceList, ResourceMeta, Workspace, WorkspaceList } from '../schema/workspace';

// listWorkspaces returns the workspaces, in order of their names
export function listWorkspaces(signal?: AbortSignal): Promise<WorkspaceList> {
    return request<WorkspaceList>('GET', '/llm/workspace', { signal });
}

// getWorkspace returns the workspace with the name
export function getWorkspace(name: string, signal?: AbortSignal): Promise<Workspace> {
    return request<Workspace>('GET', `/llm/workspace/${encodeURIComponent(name)}`, { signal });
}

// listResources returns a page of the folders, and resources, in the
// workspace's folder, from the cursor
export function listResources(name: string, path: string, cursor?: string, limit?: number, signal?: AbortSignal): Promise<ResourceList> {
    return request<ResourceList>('GET', `/llm/workspace/${encodeURIComponent(name)}/resource`, {
        query: { path: path || undefined, cursor: cursor || undefined, limit },
        signal,
    });
}

// listFolder returns every folder, and resource, in the workspace's folder,
// following the pages' cursors, up to the most, and whether there are more
export async function listFolder(name: string, path: string, most: number, signal?: AbortSignal): Promise<{ items: ResourceMeta[]; more: boolean }> {
    const items: ResourceMeta[] = [];
    let cursor: string | undefined;
    do {
        const page = await listResources(name, path, cursor, Math.min(1000, most - items.length), signal);
        items.push(...(page.body ?? []));
        cursor = page.next;
    } while (cursor && items.length < most);
    return { items, more: !!cursor };
}

// contentURL returns the URL of the content of the workspace's resource,
// which the browser shows, or plays, or, with download, saves
export async function contentURL(name: string, uri: string, download = false): Promise<string> {
    const { prefix } = await config();
    const query = new URLSearchParams({ uri });
    if (download) {
        query.set('download', 'true');
    }
    return `${prefix}/llm/workspace/${encodeURIComponent(name)}/content?${query}`;
}
