// The tools' API

import { request } from './client';
import type { NamespaceList, Tool, ToolList } from '../schema/tool';

// listTools returns the tools, of the namespace, or every one
export function listTools(namespace?: string, signal?: AbortSignal): Promise<ToolList> {
    return request<ToolList>('GET', '/llm/tool', { query: { namespace }, signal });
}

// listNamespaces returns the namespaces of the tools, with what they're for
export function listNamespaces(signal?: AbortSignal): Promise<NamespaceList> {
    return request<NamespaceList>('GET', '/llm/namespace', { signal });
}

// getTool returns the tool with the canonical name
export function getTool(name: string, signal?: AbortSignal): Promise<Tool> {
    return request<Tool>('GET', '/llm/tool/' + encodeURIComponent(name), { signal });
}
