// The workspaces' API

import { ApiError, config, request } from './client';
import type { ResourceList, ResourceMeta, Workspace, WorkspaceList } from '../schema/workspace';

// listWorkspaces returns the workspaces, in order of their names
export function listWorkspaces(signal?: AbortSignal): Promise<WorkspaceList> {
    return request<WorkspaceList>('GET', '/llm/workspace', { signal });
}

// getWorkspace returns the workspace with the name
export function getWorkspace(name: string, signal?: AbortSignal): Promise<Workspace> {
    return request<Workspace>('GET', `/llm/workspace/${encodeURIComponent(name)}`, { signal });
}

// ListOptions are which of a folder's folders, and resources, are listed:
// those under it, too, with recursive, and those of the type, such as
// FolderType, for its folders alone
export interface ListOptions {
    recursive?: boolean;
    type?: string;
}

// listResources returns a page of the folders, and resources, in the
// workspace's folder, from the cursor
export function listResources(name: string, path: string, cursor?: string, limit?: number, options: ListOptions = {}, signal?: AbortSignal): Promise<ResourceList> {
    return request<ResourceList>('GET', `/llm/workspace/${encodeURIComponent(name)}/resource`, {
        query: { path: path || undefined, cursor: cursor || undefined, limit, recursive: options.recursive || undefined, type: options.type || undefined },
        signal,
    });
}

// listFolder returns every folder, and resource, in the workspace's folder,
// or under it, following the pages' cursors, up to the most, and whether
// there are more
export async function listFolder(name: string, path: string, most: number, options: ListOptions = {}, signal?: AbortSignal): Promise<{ items: ResourceMeta[]; more: boolean }> {
    const items: ResourceMeta[] = [];
    let cursor: string | undefined;
    let more = false;
    do {
        // A server which isn't a workspace's can return more than the limit,
        // so each page is cut to what's left
        const page = await listResources(name, path, cursor, Math.min(1000, most - items.length), options, signal);
        const body = page.body ?? [];
        const left = most - items.length;
        items.push(...body.slice(0, left));
        more = body.length > left;
        cursor = page.next;
    } while (cursor && items.length < most);
    return { items, more: more || !!cursor };
}

// MarkdownPart is the first part of a resource, as Markdown: its body,
// without its front matter, and whether it has more
export interface MarkdownPart {
    body: string;
    more: boolean;
}

// readMarkdown returns the first part of the workspace's resource, as
// Markdown, of at most the limit, in bytes, or undefined, when it has none,
// such as audio. Its front matter, which says the offset of its next part,
// when it has one, is left out.
export async function readMarkdown(name: string, uri: string, limit: number, signal?: AbortSignal): Promise<MarkdownPart | undefined> {
    const { prefix } = await config();
    const query = new URLSearchParams({ uri, format: 'markdown', limit: String(limit) });
    const response = await fetch(`${prefix}/llm/workspace/${encodeURIComponent(name)}/content?${query}`, { headers: { Accept: 'text/markdown' }, signal });
    if (response.status === 501) {
        return undefined;
    } else if (!response.ok) {
        throw new ApiError(response.status, response.statusText);
    }
    const text = await response.text();
    const match = /^---\n([\s\S]*?)\n---\n?/.exec(text);
    return {
        body: match ? text.slice(match[0].length) : text,
        more: !!match && /^next:/m.test(match[1]),
    };
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
