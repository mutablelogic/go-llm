// The MCP connectors' schema, which mirrors the connectors' registry's

// Connector is a remote MCP server, as it's connected to, by its name, with
// its URL, and the patterns of the names of its tools which are included,
// and excluded, and its status: whether its server is connected, which
// server it is, and the namespace of its tools, or why it couldn't be
// connected
export interface Connector {
    name: string;
    url: string;
    include?: string[];
    exclude?: string[];
    serial?: number;
    connected?: boolean;
    server?: { name: string; version?: string };
    namespace?: string;
    error?: string;
}

// ConnectorMeta is what's set, to create, or update, a connector
export interface ConnectorMeta {
    url: string;
    include?: string[];
    exclude?: string[];
}

// ConnectorList is the connectors, and how many there are
export interface ConnectorList {
    count: number;
    body?: Connector[];
}

// connectorName returns a connector's name, from the host of its URL, such
// as seolinkmap for https://seolinkmap.com/mcp, or mcp.example for
// https://mcp.example.com, as an identifier, which starts with a letter, or
// mcp, when there's nothing of it which can be one
export function connectorName(url: string): string {
    let host = '';
    try {
        host = new URL(url.trim()).hostname.toLowerCase();
    } catch {
        // Not a URL, which the server says
    }
    const labels = host.split('.').filter((label) => label !== '' && label !== 'www');
    if (labels.length > 1) {
        labels.pop();
    }
    const name = labels.join('-').replace(/[^a-z0-9_-]/g, '').replace(/^[^a-z]+/, '').slice(0, 60);
    return name || 'mcp';
}
