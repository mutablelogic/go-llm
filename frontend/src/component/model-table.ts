// A table of models, which shows the page of models it's given, with their
// providers, and what they can do, and the pagination beneath it, which
// fires an event when the page, or the page's size, is changed, so it's
// loaded. Clicking a model fires an event, so it's shown, and the model which
// is shown is selected. It shows a skeleton until it has models.

import { LitElement, css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { capabilityLabel, modelCapabilities, type Model } from '../schema/model';
import { tableStyles } from './styles';

// Carbon components
import '@carbon/web-components/es/components/data-table/index.js';
import '@carbon/web-components/es/components/pagination/index.js';
import '@carbon/web-components/es/components/tag/tag.js';

@customElement('llm-model-table')
export class ModelTable extends LitElement {
    // The names of the events fired when the page, or its size, is changed,
    // whose detail is the page, from 1, or the size, and when a model is
    // clicked, whose detail is the model
    static readonly eventPage = 'llm-model-page';
    static readonly eventPageSize = 'llm-model-page-size';
    static readonly eventSelect = 'llm-model-select';

    // The canonical name of the model which is selected, as it's shown
    @property({ attribute: false })
    selected?: string;

    // The models on the page, which are undefined while they're loading
    @property({ attribute: false })
    models?: Model[];

    @property({ attribute: false })
    pageSizes = [10, 25, 50, 100];

    // The page, from 1, its size, and the number of models
    @property({ attribute: false })
    page = 1;

    @property({ attribute: false })
    pageSize = 25;

    @property({ attribute: false })
    total = 0;

    // Whether the server can't be reached
    @property({ type: Boolean })
    unavailable = false;

    // What's said when there are no models, such as why their provider
    // can't be reached
    @property({ attribute: false })
    empty = 'No models';

    static styles = [tableStyles, css`
        :host {
            display: block;
        }

        cds-table-row[data-selected] cds-table-cell {
            background: var(--cds-layer-selected);
        }

        .secondary {
            display: block;
            color: var(--cds-text-secondary);
            font-size: var(--cds-label-01-font-size, 0.75rem);
        }

        :host([unavailable]) cds-table {
            opacity: 0.5;
        }
    `];

    render() {
        if (!this.models) {
            return html`<cds-table-skeleton column-count="3" row-count="5"></cds-table-skeleton>`;
        }
        return html`
            <cds-table>
                <cds-table-head>
                    <cds-table-header-row>
                        <cds-table-header-cell>Model</cds-table-header-cell>
                        <cds-table-header-cell>Provider</cds-table-header-cell>
                        <cds-table-header-cell>Capabilities</cds-table-header-cell>
                    </cds-table-header-row>
                </cds-table-head>
                <cds-table-body>
                    ${this.models.length === 0
                        ? html`<cds-table-row><cds-table-cell>${this.empty}</cds-table-cell></cds-table-row>`
                        : this.models.map((model) => this.renderRow(model))}
                </cds-table-body>
            </cds-table>
            ${this.total === 0 ? nothing : html`
                <cds-pagination .page=${this.page} .pageSize=${this.pageSize} .start=${(this.page - 1) * this.pageSize}
                    .totalItems=${this.total}
                    @cds-pagination-changed-current=${(e: CustomEvent<{ page: number }>) => this.fire(ModelTable.eventPage, e.detail.page)}
                    @cds-page-sizes-select-changed=${(e: CustomEvent<{ pageSize: number }>) => this.fire(ModelTable.eventPageSize, e.detail.pageSize)}>
                    ${this.pageSizes.map((size) => html`<cds-select-item value=${size}>${size}</cds-select-item>`)}
                </cds-pagination>
            `}
        `;
    }

    private renderRow(model: Model) {
        return html`
            <cds-table-row ?data-selected=${model.canonical === this.selected} @click=${() => this.fire(ModelTable.eventSelect, model)}>
                <cds-table-cell>
                    <button class="open" type="button">${model.name}</button>
                    ${model.description ? html`<span class="secondary">${model.description}</span>` : nothing}
                </cds-table-cell>
                <cds-table-cell>${model.provider}</cds-table-cell>
                <cds-table-cell>
                    ${modelCapabilities(model).map((name) => html`<cds-tag size="sm" type="cool-gray">${capabilityLabel(name)}</cds-tag>`)}
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
        'llm-model-table': ModelTable;
    }
}
