// A table of tools, and agents, which are tools in the agent namespace,
// which shows the page of tools it's given, with their namespaces, names,
// and descriptions, beneath their titles, when they have them, such as
// agents, and the pagination beneath it, which fires an event when the
// page, or the page's size, is changed. Clicking a
// tool fires an event, so it's shown, and the tool which is shown is
// selected. It shows a skeleton until it has tools.

import { LitElement, css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { toolNamespace, type Tool } from '../schema/tool';
import { tableStyles } from './styles';

// Carbon components
import '@carbon/web-components/es/components/data-table/index.js';
import '@carbon/web-components/es/components/pagination/index.js';
import '@carbon/web-components/es/components/tag/tag.js';

@customElement('llm-tool-table')
export class ToolTable extends LitElement {
    // The names of the events fired when the page, or its size, is changed,
    // and when a tool is clicked, whose detail is the tool
    static readonly eventPage = 'llm-tool-page';
    static readonly eventPageSize = 'llm-tool-page-size';
    static readonly eventSelect = 'llm-tool-select';

    // The canonical name of the tool which is selected, as it's shown
    @property({ attribute: false })
    selected?: string;

    // The tools on the page, which are undefined while they're loading
    @property({ attribute: false })
    tools?: Tool[];

    @property({ attribute: false })
    pageSizes = [10, 25, 50, 100];

    @property({ attribute: false })
    page = 1;

    @property({ attribute: false })
    pageSize = 25;

    @property({ attribute: false })
    total = 0;

    // Whether the server can't be reached
    @property({ type: Boolean })
    unavailable = false;

    static styles = [tableStyles, css`
        :host {
            display: block;
        }

        cds-table-row[data-selected] cds-table-cell {
            background: var(--cds-layer-selected);
        }

        .title {
            display: block;
        }

        /* A tool's description, beneath its title, is at most two lines */
        .secondary {
            display: -webkit-box;
            -webkit-box-orient: vertical;
            -webkit-line-clamp: 2;
            overflow: hidden;
            color: var(--cds-text-secondary);
            font-size: var(--cds-label-01-font-size, 0.75rem);
        }

        :host([unavailable]) cds-table {
            opacity: 0.5;
        }
    `];

    render() {
        if (!this.tools) {
            return html`<cds-table-skeleton column-count="3" row-count="5"></cds-table-skeleton>`;
        }
        return html`
            <cds-table>
                <cds-table-head>
                    <cds-table-header-row>
                        <cds-table-header-cell>Namespace</cds-table-header-cell>
                        <cds-table-header-cell>Tool</cds-table-header-cell>
                        <cds-table-header-cell>Description</cds-table-header-cell>
                    </cds-table-header-row>
                </cds-table-head>
                <cds-table-body>
                    ${this.tools.length === 0
                        ? html`<cds-table-row><cds-table-cell>No tools</cds-table-cell></cds-table-row>`
                        : this.tools.map((tool) => this.renderRow(tool))}
                </cds-table-body>
            </cds-table>
            ${this.total === 0 ? nothing : html`
                <cds-pagination .page=${this.page} .pageSize=${this.pageSize} .start=${(this.page - 1) * this.pageSize}
                    .totalItems=${this.total}
                    @cds-pagination-changed-current=${(e: CustomEvent<{ page: number }>) => this.fire(ToolTable.eventPage, e.detail.page)}
                    @cds-page-sizes-select-changed=${(e: CustomEvent<{ pageSize: number }>) => this.fire(ToolTable.eventPageSize, e.detail.pageSize)}>
                    ${this.pageSizes.map((size) => html`<cds-select-item value=${size}>${size}</cds-select-item>`)}
                </cds-pagination>
            `}
        `;
    }

    private renderRow(tool: Tool) {
        // The description, apart from when it's the title, as it is of an
        // agent without a description
        const title = (tool.title ?? '').trim();
        const summary = (tool.description ?? '').replace(/\s+/g, ' ').trim();
        const description = summary === title ? '' : summary;
        const namespace = toolNamespace(tool);
        return html`
            <cds-table-row ?data-selected=${tool.canonical === this.selected} @click=${() => this.fire(ToolTable.eventSelect, tool)}>
                <cds-table-cell>
                    <cds-tag size="sm" type=${namespace === 'agent' ? 'purple' : 'cool-gray'}>${namespace}</cds-tag>
                </cds-table-cell>
                <cds-table-cell>
                    <button class="open" type="button">${tool.name}</button>
                </cds-table-cell>
                <cds-table-cell>
                    ${title ? html`<span class="title">${title}</span>` : nothing}
                    ${description ? html`<span class="secondary">${description}</span>` : nothing}
                </cds-table-cell>
            </cds-table-row>
        `;
    }

    private fire(name: string, detail: unknown) {
        this.dispatchEvent(new CustomEvent(name, { detail }));
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-tool-table': ToolTable;
    }
}
