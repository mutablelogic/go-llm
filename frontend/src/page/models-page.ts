// The page of the models, of every provider, or of one, with actions which
// add a provider, when every provider's are shown, or edit, and delete, the
// one which is chosen, and load them again

import { css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import MachineLearningModel32 from '@carbon/icons/es/machine-learning-model/32.js';
import Edit16 from '@carbon/icons/es/edit/16.js';
import TrashCan16 from '@carbon/icons/es/trash-can/16.js';
import Download16 from '@carbon/icons/es/download/16.js';
import { Page } from './page';
import { presetIcon, presets, type Preset } from '../schema/preset';
import Api16 from '@carbon/icons/es/api/16.js';
import Microphone16 from '@carbon/icons/es/microphone/16.js';
import ChatBot16 from '@carbon/icons/es/chat-bot/16.js';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/menu-button/index.js';
import '@carbon/web-components/es/components/menu/index.js';

@customElement('llm-models-page')
export class ModelsPage extends Page {
    // A provider's logo is on a small white tile, so a dark one, such as
    // OpenAI's, is seen in the dark theme too
    static styles = [Page.styles, css`
        img.brand {
            box-sizing: border-box;
            width: 1rem;
            height: 1rem;
            padding: 1px;
            border-radius: 3px;
            background: #fff;
            object-fit: contain;
        }
    `];

    // The names of the events fired to add a provider, of the preset which is
    // the detail, and to edit, and delete, the one which is chosen
    static readonly eventAdd = 'llm-models-page-add';
    static readonly eventEdit = 'llm-models-page-edit';
    static readonly eventDelete = 'llm-models-page-delete';
    static readonly eventLoad = 'llm-models-page-load';

    heading = 'Models';
    description = 'The models that power the assistant';

    // The provider which is chosen, whose models are shown, which can be
    // edited, and deleted, or none, for every provider's
    @property({ attribute: false })
    provider?: string;

    // Whether the provider which is chosen stores models, so a model can be
    // loaded with it, which is downloaded, when it isn't stored
    @property({ type: Boolean })
    loadable = false;

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
            ${this.provider ? html`
                ${this.loadable ? html`
                    <cds-button kind="primary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(ModelsPage.eventLoad, { detail: this.provider }))}>
                        Load model ${iconLoader(Download16, { slot: 'icon' })}
                    </cds-button>
                ` : nothing}
            ` : html`
                <cds-menu-button label="Add provider" kind="primary" size="md">
                    <cds-menu size="md">
                        ${presets.map((preset) => html`
                            <cds-menu-item label=${preset.label}
                                @click=${() => this.dispatchEvent(new CustomEvent(ModelsPage.eventAdd, { detail: preset.id }))}>
                                ${menuIcon(preset)}
                            </cds-menu-item>
                        `)}
                    </cds-menu>
                </cds-menu-button>
            `}
        `;
    }
}

// menuIcon returns the icon of the preset in the menu which adds a
// provider: its brand's logo, or an icon of what it is, such as a server
// with an OpenAI-compatible API, or speech, or a chatbot
function menuIcon(preset: Preset) {
    const icon = presetIcon(preset);
    if (icon) {
        return html`<img slot="render-icon" class="brand" src=${icon} alt="">`;
    }
    switch (preset.id) {
    case 'gowhisper':
        return iconLoader(Microphone16, { slot: 'render-icon' });
    case 'eliza':
        return iconLoader(ChatBot16, { slot: 'render-icon' });
    default:
        return iconLoader(Api16, { slot: 'render-icon' });
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-models-page': ModelsPage;
    }
}
