// The MCP connectors' schema, which mirrors the connectors' registry's

// Connector is a remote MCP server, as it's connected to, by its name, with
// its URL, the patterns of the names of its tools which are included, and
// excluded, and the names of the credentials of its own client, when it has
// one, and its status: whether its server is connected, which server it is,
// and the namespace of its tools, or why it couldn't be connected, and
// whether it needs to be signed in to, or is
export interface Connector {
    name: string;
    url: string;
    include?: string[];
    exclude?: string[];
    client_id_credential?: string;
    client_secret_credential?: string;
    auth?: 'required' | 'authorized';
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
    client_id_credential?: string;
    client_secret_credential?: string;
}

// needsSignIn returns true when the connector's server needs to be signed
// in to, before it's connected
export function needsSignIn(connector: Connector): boolean {
    return !connector.connected && connector.auth === 'required';
}

// ConnectorList is the connectors, and how many there are
export interface ConnectorList {
    count: number;
    body?: Connector[];
}

// ConnectorTemplate is a remote MCP server which a connector can be added
// for, without typing its URL, such as GitHub's, which the server offers,
// as it has the credentials, whose names it has, of the client it's signed
// in to with
export interface ConnectorTemplate {
    name: string;
    description?: string;
    url: string;
    client_id_credential?: string;
    client_secret_credential?: string;
    icon?: string;
}

// connectorIcon returns the URL of the template's logo, among the web UI's
// connector icons, or undefined, when it has none
export function connectorIcon(template: ConnectorTemplate): string | undefined {
    return template.icon ? 'icon/connector/' + encodeURIComponent(template.icon) : undefined;
}

// ConnectorTemplateList is the templates, and how many there are
export interface ConnectorTemplateList {
    count: number;
    body?: ConnectorTemplate[];
}
