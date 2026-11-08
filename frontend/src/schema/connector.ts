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
