// A picker of the tools a session's model can call, in a hierarchy: every
// tool, every tool in a namespace, or each tool, with a checkbox for each,
// of which a namespace's, and every tool's, are partly checked when only
// some of what's beneath them are. Its value is the names it chooses, as
// the registry takes them: * for every tool, a namespace and *, such as
// weatherapi:*, for every tool in it, and each tool's canonical name, such as
// homeassistant:query, otherwise, so tools which are added later, to every
// namespace, or to a namespace, are included.

import { LitElement, css, html, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import ChevronRight16 from '@carbon/icons/es/chevron--right/16.js';
import { live } from 'lit/directives/live.js';
import { toolNamespace, type Namespace, type Tool } from '../schema/tool';

// hidden are the tools which aren't chosen, as only the server runs them,
// which the toolkit leaves out of * and builtin:* too
const hidden = new Set(['builtin:event', 'builtin:compact']);

// groupOf returns the group a namespace is shown in, which is agent for the
// agents' namespaces, those which people make, and the server's own, so
// they're chosen together, or otherwise the namespace
function groupOf(namespace: string): string {
    return namespace === 'builtin' ? 'agent' : namespace;
}

// Carbon components
import '@carbon/web-components/es/components/checkbox/index.js';
import '@carbon/web-components/es/components/tag/index.js';
import '@carbon/web-components/es/components/skeleton-text/index.js';

@customElement('llm-tool-picker')
export class ToolPicker extends LitElement {
    // The tools, which are undefined while they're loading
    @property({ attribute: false })
    tools?: Tool[];

    // The namespaces, with what their tools are for, which are shown beneath
    // them
    @property({ attribute: false })
    described: Namespace[] = [];

    // The names which are chosen, which is every tool, unless it's set
    @property({ attribute: false })
    value: string[] = ['*'];

    // The namespaces which are expanded, to show their tools, which none are
    // at first
    @state()
    private expanded = new Set<string>();

    static styles = css`
        :host {
            display: block;
        }

        .label {
            margin-bottom: 0.5rem;
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
        }

        ul {
            margin: 0;
            padding: 0;
            list-style: none;
        }

        /* A namespace is a row: a button which expands it, its checkbox, and
           how many of its tools are chosen */
        .namespace {
            display: flex;
            align-items: center;
            gap: 0.25rem;
            border-top: 1px solid var(--cds-border-subtle-01);
        }

        .namespace cds-checkbox {
            flex: 1;
        }

        .expand {
            all: unset;
            display: flex;
            align-items: center;
            justify-content: center;
            width: 2rem;
            height: 2rem;
            cursor: pointer;
        }

        .expand:focus-visible {
            outline: 2px solid var(--cds-focus);
            outline-offset: -2px;
        }

        .expand svg {
            fill: currentColor;
            transition: transform 110ms;
        }

        .expand[aria-expanded='true'] svg {
            transform: rotate(90deg);
        }

        .about {
            display: block;
            margin: -0.5rem 0 0.25rem 3.625rem;
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
        }

        .count {
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
            font-variant-numeric: tabular-nums;
            white-space: nowrap;
        }

        /* A namespace's tools are beneath it, indented */
        .tools {
            padding: 0 0 0.5rem 4.25rem;
        }

        /* A tool's checkbox, and its badge, after its name, on one line */
        .tool {
            display: flex;
            align-items: center;
            gap: 0.25rem;
        }

        .tool cds-tag {
            margin: 0;
        }

        .description {
            display: block;
            margin: -0.25rem 0 0.25rem 1.625rem;
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
    `;

    render() {
        if (!this.tools) {
            return html`<div class="label">Tools</div><cds-skeleton-text paragraph line-count="3"></cds-skeleton-text>`;
        }
        const groups = this.groups();
        const any = this.value.length > 0;
        return html`
            <div class="label">Tools</div>
            <cds-checkbox .checked=${live(this.everything())} .indeterminate=${live(!this.everything() && any)}
                @cds-checkbox-changed=${(e: Event) => this.toggleEverything(e)}>Every tool, and agent</cds-checkbox>
            <ul>
                ${[...groups].map(([namespace, tools]) => {
                    const expanded = this.expanded.has(namespace);
                    const chosen = tools.filter((tool) => this.toolChosen(tool)).length;
                    return html`
                    <li>
                        <div class="namespace">
                            <button class="expand" type="button" aria-expanded=${String(expanded)} aria-label=${`Show the tools of ${namespace}`}
                                @click=${() => this.toggleExpanded(namespace)}>${iconLoader(ChevronRight16)}</button>
                            <cds-checkbox .checked=${live(this.groupChosen(namespace))}
                                .indeterminate=${live(!this.groupChosen(namespace) && chosen > 0)}
                                @cds-checkbox-changed=${(e: Event) => this.toggleGroup(e, namespace)}>${namespace}</cds-checkbox>
                            <span class="count">${chosen} of ${tools.length}</span>
                        </div>
                        ${this.about(namespace) ? html`<span class="about">${this.about(namespace)}</span>` : nothing}
                        ${expanded ? html`<ul class="tools">
                            ${tools.map((tool) => html`
                                <li>
                                    <div class="tool">
                                        <cds-checkbox .checked=${live(this.toolChosen(tool))}
                                            @cds-checkbox-changed=${(e: Event) => this.toggleTool(e, tool)}>${tool.name}</cds-checkbox>
                                        ${tool.agent ? html`<cds-tag size="sm" type="purple">Agent</cds-tag>` : nothing}
                                    </div>
                                    ${tool.description ? html`<span class="description" title=${tool.description}>${tool.description.replace(/\s+/g, ' ').trim()}</span>` : nothing}
                                </li>
                            `)}
                        </ul>` : nothing}
                    </li>
                `;
                })}
            </ul>
        `;
    }

    // namespaces returns the tools which can be chosen, by their namespaces,
    // in order of their namespaces' names, which are what's chosen
    private namespaces(): Map<string, Tool[]> {
        const namespaces = new Map<string, Tool[]>();
        for (const tool of [...(this.tools ?? [])].sort((a, b) => a.canonical.localeCompare(b.canonical))) {
            if (hidden.has(tool.canonical)) {
                continue;
            }
            const namespace = toolNamespace(tool);
            namespaces.set(namespace, [...(namespaces.get(namespace) ?? []), tool]);
        }
        return namespaces;
    }

    // groups returns the tools which can be chosen, by the groups they're
    // shown in, in order of their names, with the agents' namespaces as one
    private groups(): Map<string, Tool[]> {
        const groups = new Map<string, Tool[]>();
        for (const [namespace, tools] of this.namespaces()) {
            const group = groupOf(namespace);
            groups.set(group, [...(groups.get(group) ?? []), ...tools]);
        }
        return new Map([...groups].sort(([a], [b]) => a.localeCompare(b)));
    }

    // groupNamespaces returns the namespaces of the group, which have tools
    private groupNamespaces(group: string): string[] {
        return [...this.namespaces().keys()].filter((namespace) => groupOf(namespace) === group);
    }

    // about returns what the namespace's tools are for, when it says
    private about(namespace: string): string | undefined {
        return this.described.find((other) => other.name === namespace)?.description;
    }

    // toggleExpanded shows the namespace's tools, or hides them
    private toggleExpanded(namespace: string) {
        const expanded = new Set(this.expanded);
        if (!expanded.delete(namespace)) {
            expanded.add(namespace);
        }
        this.expanded = expanded;
    }

    private everything(): boolean {
        return this.value.includes('*');
    }

    private namespaceChosen(namespace: string): boolean {
        return this.everything() || this.value.includes(`${namespace}:*`);
    }

    // groupChosen returns whether every namespace of the group is chosen
    private groupChosen(group: string): boolean {
        const namespaces = this.groupNamespaces(group);
        return namespaces.length > 0 && namespaces.every((namespace) => this.namespaceChosen(namespace));
    }

    private toolChosen(tool: Tool): boolean {
        return this.namespaceChosen(toolNamespace(tool)) || this.value.includes(tool.canonical);
    }

    // toggleEverything chooses every tool, or none
    private toggleEverything(event: Event) {
        event.stopPropagation();
        this.choose(this.everything() ? [] : ['*']);
    }

    // toggleGroup chooses every tool in the group's namespaces, or none of
    // them, keeping the other namespaces as they were, when every tool was
    // chosen
    private toggleGroup(event: Event, group: string) {
        event.stopPropagation();
        const namespaces = [...this.namespaces().keys()];
        let value = this.everything() ? namespaces.map((name) => `${name}:*`) : [...this.value];
        const chosen = this.groupChosen(group);
        for (const namespace of this.groupNamespaces(group)) {
            value = value.filter((name) => !name.startsWith(`${namespace}:`));
            if (!chosen) {
                value.push(`${namespace}:*`);
            }
        }
        this.choose(value);
    }

    // toggleTool chooses the tool, or not, keeping the other tools in its
    // namespace, and the other namespaces, as they were, when they were
    // chosen as a whole
    private toggleTool(event: Event, tool: Tool) {
        event.stopPropagation();
        const namespaces = this.namespaces();
        const namespace = toolNamespace(tool);
        let value = this.everything() ? [...namespaces.keys()].map((name) => `${name}:*`) : [...this.value];
        if (value.includes(`${namespace}:*`)) {
            value = value.filter((name) => name !== `${namespace}:*`);
            value.push(...(namespaces.get(namespace) ?? []).map((other) => other.canonical));
        }
        if (this.toolChosen(tool)) {
            value = value.filter((name) => name !== tool.canonical);
        } else {
            value.push(tool.canonical);
        }
        this.choose(value);
    }

    // choose sets the value, as generally as it can be: a namespace whose
    // tools are all chosen is chosen as a whole, and every tool is, when
    // every namespace is
    private choose(value: string[]) {
        const namespaces = this.namespaces();
        for (const [namespace, tools] of namespaces) {
            if (!value.includes(`${namespace}:*`) && tools.every((tool) => value.includes(tool.canonical))) {
                value = value.filter((name) => !name.startsWith(`${namespace}:`));
                value.push(`${namespace}:*`);
            }
        }
        if (namespaces.size > 0 && [...namespaces.keys()].every((namespace) => value.includes(`${namespace}:*`))) {
            value = ['*'];
        }
        this.value = [...new Set(value)].sort();
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-tool-picker': ToolPicker;
    }
}
