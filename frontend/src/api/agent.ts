// The agents' API, whose definitions are read, and written, as YAML, as an
// agent's file is written, with the comments, and order, it's written in

import { ApiError, config, request } from './client';

// Agent is an agent, by its name, and whether it's built in, so it can't be
// changed, or deleted, and internal, so it isn't a tool
export interface Agent {
    name: string;
    title?: string;
    description?: string;
    serial?: number;
    builtin?: boolean;
    internal?: boolean;
    // Its model, or none, for the model of the session it's run from, which
    // is all of a built-in agent which can be changed
    model?: string;
}

// yamlType is the content type of an agent's definition, as YAML
const yamlType = 'application/yaml';

// getAgent returns the agent with the name
export function getAgent(name: string, signal?: AbortSignal): Promise<Agent> {
    return request<Agent>('GET', '/llm/agent/' + encodeURIComponent(name), { signal });
}

// getAgentYAML returns the definition of the agent with the name, as YAML
export async function getAgentYAML(name: string, signal?: AbortSignal): Promise<string> {
    const { prefix } = await config();
    const response = await fetch(prefix + '/llm/agent/' + encodeURIComponent(name), { headers: { Accept: yamlType }, signal });
    if (!response.ok) {
        throw await apiError(response);
    }
    return response.text();
}

// saveAgentYAML creates the agent with the definition, as YAML, or replaces
// the agent with its name, and returns it
export async function saveAgentYAML(yaml: string): Promise<Agent> {
    const { prefix } = await config();
    const response = await fetch(prefix + '/llm/agent', {
        method: 'POST',
        headers: { 'Content-Type': yamlType, Accept: 'application/json' },
        body: yaml,
    });
    if (!response.ok) {
        throw await apiError(response);
    }
    return response.json() as Promise<Agent>;
}

// updateAgentModel sets the model of the agent with the name, such as a
// built-in agent's, or clears it, with an empty one, so it's run with the
// model of the session it's run from
export function updateAgentModel(name: string, model: string): Promise<Agent> {
    return request<Agent>('PATCH', '/llm/agent/' + encodeURIComponent(name), { body: { model } });
}

// deleteAgent deletes the agent with the name
export function deleteAgent(name: string): Promise<Agent> {
    return request<Agent>('DELETE', '/llm/agent/' + encodeURIComponent(name));
}

// apiError returns the error of a response, which the server returns as
// JSON, but a proxy may not
async function apiError(response: Response): Promise<ApiError> {
    const error = await response.json().catch(() => undefined);
    return new ApiError(response.status, error?.reason || response.statusText, error?.detail);
}
