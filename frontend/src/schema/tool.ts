// The tools' schema, which mirrors the toolkit's

// NamespaceType is where a namespace's tools are from: built in, an MCP
// server, which is a connector, or the agents
export type NamespaceType = 'builtin' | 'mcp' | 'agent';

// Tool is a tool which models can call, in a namespace, such as
// weatherapi:current, or an agent, in the agent namespace, with its title,
// for people, which only some tools, such as agents, have, and the JSON
// schemas of its input and output
export interface Tool {
    name: string;
    canonical: string;
    type: NamespaceType;
    title?: string;
    // Whether it's an agent, which asks a model to do what it does, whatever
    // its namespace
    agent?: boolean;
    description?: string;
    input?: unknown;
    output?: unknown;
}

// Namespace is a namespace of tools, where they're from, with a line which
// says what its tools are for, when it has one, and how many tools it has
export interface Namespace {
    name: string;
    type: NamespaceType;
    description?: string;
    tools: number;
}

// NamespaceList is the namespaces, and how many there are
export interface NamespaceList {
    count: number;
    body?: Namespace[];
}

// ToolList is the tools, and how many there are
export interface ToolList {
    count: number;
    body?: Tool[];
}

// toolNamespace returns the namespace of the tool, such as weatherapi
export function toolNamespace(tool: Tool): string {
    const colon = tool.canonical.indexOf(':');
    return colon < 0 ? '' : tool.canonical.slice(0, colon);
}
