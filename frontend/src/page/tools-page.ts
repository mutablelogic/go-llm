// The page of the tools, and the agents, which models can call, with a menu
// which adds a connector, whose tools are added, such as a custom one, by
// its URL, when every tool is shown, and, when the agents are shown, an
// action which starts a new agent

import { html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import ToolKit32 from '@carbon/icons/es/tool-kit/32.js';
import Add16 from '@carbon/icons/es/add/16.js';
import { Page } from './page';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/menu-button/index.js';
import '@carbon/web-components/es/components/menu/index.js';

@customElement('llm-tools-page')
export class ToolsPage extends Page {
    // The names of the events fired to start a new agent, and to add a
    // connector, of the kind which is the detail, such as custom
    static readonly eventNewAgent = 'llm-tools-page-new-agent';
    static readonly eventAddConnector = 'llm-tools-page-add-connector';

    heading = 'Tools';
    description = 'The tools, and agents, which models can call';

    // The namespace whose tools are shown, or none, for every tool
    @property({ attribute: false })
    namespace?: string;

    protected renderIcon(): unknown {
        return iconLoader(ToolKit32);
    }

    protected renderActions(): unknown {
        const agents = this.namespace === 'agent';
        return html`
            ${agents ? html`
                <cds-button kind="primary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(ToolsPage.eventNewAgent))}>
                    New agent ${iconLoader(Add16, { slot: 'icon' })}
                </cds-button>
            ` : nothing}
            ${this.namespace ? nothing : html`
                <cds-menu-button label="Add connector" kind="primary" size="md">
                    <cds-menu size="md">
                        <cds-menu-item label="Custom connector"
                            @click=${() => this.dispatchEvent(new CustomEvent(ToolsPage.eventAddConnector, { detail: 'custom' }))}></cds-menu-item>
                    </cds-menu>
                </cds-menu-button>
            `}
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-tools-page': ToolsPage;
    }
}
