// A panel which shows a model: its provider, canonical name and aliases, its
// token limits, what it can do, and what its provider says of it, with
// actions which start a chat with it, and, of a provider which stores
// models, such as Ollama, delete it

import { css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { Panel, formatValue } from './panel';
import { capabilityNames, cannot, type Model } from '../schema/model';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Chat16 from '@carbon/icons/es/chat/16.js';
import TrashCan16 from '@carbon/icons/es/trash-can/16.js';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';
import { formatTime } from '../format';

@customElement('llm-model-view')
export class ModelView extends Panel<Model> {
    // The name of the event fired to start a new chat with the model, whose
    // detail is its canonical name
    static readonly eventChat = 'llm-model-chat';

    // The name of the event fired to delete the model, whose detail is its
    // canonical name
    static readonly eventDelete = 'llm-model-delete';

    // The model which is shown, or none, when the panel is closed
    @property({ attribute: false })
    model?: Model;

    // Whether its provider stores models, so it can be deleted, and whether
    // it's being
    @property({ type: Boolean })
    stored = false;

    @property({ type: Boolean })
    busy = false;

    static styles = [Panel.styles, css`
        /* The actions are side by side, and wrap */
        .actions {
            display: flex;
            flex-wrap: wrap;
            gap: 1px;
        }
    `];

    protected current() {
        return this.model;
    }

    protected renderTitle(model: Model) {
        return model.name;
    }

    protected renderSubtitle(model: Model) {
        return model.canonical;
    }

    protected renderBody(model: Model) {
        const meta = Object.entries(model.meta ?? {});
        return html`
            ${model.description ? html`<p>${model.description}</p>` : nothing}
            ${cannot(model, 'chat') && !this.stored ? nothing : html`
                <p class="actions">
                    ${cannot(model, 'chat') ? nothing : html`
                        <cds-button kind="primary" size="md"
                            @click=${() => this.dispatchEvent(new CustomEvent(ModelView.eventChat, { detail: model.canonical }))}>
                            New chat ${iconLoader(Chat16, { slot: 'icon' })}
                        </cds-button>
                    `}
                    ${this.stored ? html`
                        <cds-button kind="danger" size="md" ?disabled=${this.busy}
                            @click=${() => this.dispatchEvent(new CustomEvent(ModelView.eventDelete, { detail: model.canonical }))}>
                            Delete ${iconLoader(TrashCan16, { slot: 'icon' })}
                        </cds-button>
                    ` : nothing}
                </p>
            `}
            <h3>Model</h3>
            <dl>
                <dt>Provider</dt><dd>${model.provider}</dd>
                <dt>Name</dt><dd>${model.canonical}</dd>
                ${model.aliases?.length ? html`<dt>Aliases</dt><dd>${model.aliases.join(', ')}</dd>` : nothing}
                ${model.created ? html`<dt>Created</dt><dd>${formatTime(model.created)}</dd>` : nothing}
                ${model.input_token_limit ? html`<dt>Input tokens</dt><dd>${model.input_token_limit.toLocaleString()}</dd>` : nothing}
                ${model.output_token_limit ? html`<dt>Output tokens</dt><dd>${model.output_token_limit.toLocaleString()}</dd>` : nothing}
            </dl>
            <h3>Capabilities</h3>
            <dl>${[...capabilityNames, 'temperature' as const].map((name) => {
                const can = model.capabilities?.[name];
                return html`<dt>${name}</dt><dd>${can === true ? 'Yes' : can === false ? 'No' : 'Unknown'}</dd>`;
            })}</dl>
            ${meta.length ? html`
                <h3>Metadata</h3>
                <dl>${meta.map(([key, value]) => html`<dt>${key}</dt><dd>${formatValue(value)}</dd>`)}</dl>
            ` : nothing}
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-model-view': ModelView;
    }
}
