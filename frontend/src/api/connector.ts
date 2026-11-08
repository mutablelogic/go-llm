// The MCP connectors' API

import { request } from './client';
import type { Connector, ConnectorList, ConnectorMeta, ConnectorTemplateList } from '../schema/connector';

// listConnectors returns the connectors, with their status
export function listConnectors(signal?: AbortSignal): Promise<ConnectorList> {
    return request<ConnectorList>('GET', '/mcp/connector', { signal });
}

// getConnector returns the connector, with its status
export function getConnector(name: string, signal?: AbortSignal): Promise<Connector> {
    return request<Connector>('GET', '/mcp/connector/' + encodeURIComponent(name), { signal });
}

// createConnector creates a connector, which the server connects to, and
// names after its server, as it says, with a number added, when that's
// another connector's
export function createConnector(meta: ConnectorMeta): Promise<Connector> {
    return request<Connector>('POST', '/mcp/connector', { body: meta });
}

// updateConnector replaces the connector's URL, and patterns, once the
// server at the URL is connected to, unless it's the same, and connected,
// when its tools are added again, with its patterns
export function updateConnector(name: string, meta: ConnectorMeta): Promise<Connector> {
    return request<Connector>('PATCH', '/mcp/connector/' + encodeURIComponent(name), { body: meta });
}

// deleteConnector deletes the connector, which is disconnected
export function deleteConnector(name: string): Promise<Connector> {
    return request<Connector>('DELETE', '/mcp/connector/' + encodeURIComponent(name));
}

// authorizeConnector starts signing in to the connector's server, and
// returns the URL a person signs in at, which redirects back to the server,
// at the browser's origin, once they have
export async function authorizeConnector(name: string): Promise<string> {
    const response = await request<{ url: string }>('POST', '/mcp/connector/' + encodeURIComponent(name) + '/authorize', { body: {} });
    return response.url;
}

// listTemplates returns the templates of the remote MCP servers which a
// connector can be added for, which the server offers, apart from those
// which are connectors already
export function listTemplates(signal?: AbortSignal): Promise<ConnectorTemplateList> {
    return request<ConnectorTemplateList>('GET', '/mcp/template', { signal });
}
