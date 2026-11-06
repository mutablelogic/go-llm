// The page of the models, of every provider, or of one, with actions which
// add a provider, when every provider's are shown, or edit, and delete, the
// one which is chosen, and load them again

import { html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import MachineLearningModel32 from '@carbon/icons/es/machine-learning-model/32.js';
import Add16 from '@carbon/icons/es/add/16.js';
import Edit16 from '@carbon/icons/es/edit/16.js';
import TrashCan16 from '@carbon/icons/es/trash-can/16.js';
import { Page } from './page';
import { refreshAction } from './refresh';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';

@customElement('llm-models-page')
export class ModelsPage extends Page {
    // The names of the events fired to add a provider, and to edit, and
    // delete, the one which is chosen
    static readonly eventAdd = 'llm-models-page-add';
    static readonly eventEdit = 'llm-models-page-edit';
    static readonly eventDelete = 'llm-models-page-delete';

    heading = 'Models';
    description = 'The models of the providers, and what they can do';

    // The provider which is chosen, whose models are shown, which can be
    // edited, and deleted, or none, for every provider's
    @property({ attribute: false })
    provider?: string;

    protected renderIcon(): unknown {
        return iconLoader(MachineLearningModel32);
    }

    protected renderActions(): unknown {
        return html`
            ${this.provider ? html`
                <cds-button kind="secondary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(ModelsPage.eventEdit, { detail: this.provider }))}>
                    Edit ${this.provider} ${iconLoader(Edit16, { slot: 'icon' })}
                </cds-button>
                <cds-button kind="secondary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(ModelsPage.eventDelete, { detail: this.provider }))}>
                    Delete ${iconLoader(TrashCan16, { slot: 'icon' })}
                </cds-button>
            ` : nothing}
            ${this.provider ? refreshAction(this) : html`
                ${refreshAction(this, 'secondary')}
                <cds-button kind="primary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(ModelsPage.eventAdd))}>
                    Add provider ${iconLoader(Add16, { slot: 'icon' })}
                </cds-button>
            `}
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-models-page': ModelsPage;
    }
}
