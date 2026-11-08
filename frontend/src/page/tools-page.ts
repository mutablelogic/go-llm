// The page of the tools, and the agents, which models can call, with an
// action which adds an MCP connector, whose tools are added, when every
// tool is shown, and, when the agents are shown, one which starts a new
// agent

import { html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import ToolKit32 from '@carbon/icons/es/tool-kit/32.js';
import Add16 from '@carbon/icons/es/add/16.js';
import { Page } from './page';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';

@customElement('llm-tools-page')
export class ToolsPage extends Page {
    // The names of the events fired to start a new agent, and to add an MCP
    // connector
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
                <cds-button kind="primary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(ToolsPage.eventAddConnector))}>
                    Add MCP connector ${iconLoader(Add16, { slot: 'icon' })}
                </cds-button>
            `}
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-tools-page': ToolsPage;
    }
}
