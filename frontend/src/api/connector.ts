// The MCP connectors' API

import { request } from './client';
import type { Connector, ConnectorList, ConnectorMeta } from '../schema/connector';

// listConnectors returns the connectors, with their status
export function listConnectors(signal?: AbortSignal): Promise<ConnectorList> {
    return request<ConnectorList>('GET', '/mcp/connector', { signal });
}

// getConnector returns the connector, with its status
export function getConnector(name: string, signal?: AbortSignal): Promise<Connector> {
    return request<Connector>('GET', '/mcp/connector/' + encodeURIComponent(name), { signal });
}

// createConnector creates a connector, with its name, which the server
// connects to
export function createConnector(name: string, meta: ConnectorMeta): Promise<Connector> {
    return request<Connector>('POST', '/mcp/connector', { body: { name, ...meta } });
}

// deleteConnector deletes the connector, which is disconnected
export function deleteConnector(name: string): Promise<Connector> {
    return request<Connector>('DELETE', '/mcp/connector/' + encodeURIComponent(name));
}
