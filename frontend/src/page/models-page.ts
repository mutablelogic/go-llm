// The page of the models, of every provider, or of one, with actions which
// add a provider, when every provider's are shown, or edit, and delete, the
// one which is chosen, and load them again

import { html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import MachineLearningModel32 from '@carbon/icons/es/machine-learning-model/32.js';
import Edit16 from '@carbon/icons/es/edit/16.js';
import TrashCan16 from '@carbon/icons/es/trash-can/16.js';
import { Page } from './page';
import { refreshAction } from './refresh';
import { presets } from '../schema/preset';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/menu-button/index.js';
import '@carbon/web-components/es/components/menu/index.js';

@customElement('llm-models-page')
export class ModelsPage extends Page {
    // The names of the events fired to add a provider, of the preset which is
    // the detail, and to edit, and delete, the one which is chosen
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
                <cds-menu-button label="Add provider" kind="primary" size="md">
                    <cds-menu size="md">
                        ${presets.map((preset) => html`
                            <cds-menu-item label=${preset.label}
                                @click=${() => this.dispatchEvent(new CustomEvent(ModelsPage.eventAdd, { detail: preset.id }))}></cds-menu-item>
                        `)}
                    </cds-menu>
                </cds-menu-button>
            `}
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-models-page': ModelsPage;
    }
}
