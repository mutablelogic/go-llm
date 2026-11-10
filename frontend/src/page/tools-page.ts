// The page of the tools, and the agents, which models can call, with a menu
// which adds a connector, whose tools are added, such as a custom one, by
// its URL, when every tool is shown, and, when the agents are shown, an
// action which starts a new agent

import { css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import ToolKit32 from '@carbon/icons/es/tool-kit/32.js';
import Add16 from '@carbon/icons/es/add/16.js';
import Edit16 from '@carbon/icons/es/edit/16.js';
import TrashCan16 from '@carbon/icons/es/trash-can/16.js';
import { Page } from './page';
import { connectorIcon, type ConnectorTemplate } from '../schema/connector';
import LogoMCP16 from '@carbon/icons/es/logo--model-context-protocol/16.js';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/menu-button/index.js';
import '@carbon/web-components/es/components/menu/index.js';

@customElement('llm-tools-page')
export class ToolsPage extends Page {
    // A connector's logo is on a small white tile, so a dark one, such as
    // GitHub's, is seen in the dark theme too
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

    // The names of the events fired to start a new agent, to add a
    // connector, of the template which is the detail, or a custom one,
    // without one, and to edit,
    // and delete, the connector of the namespace which is chosen
    static readonly eventNewAgent = 'llm-tools-page-new-agent';
    static readonly eventAddConnector = 'llm-tools-page-add-connector';
    static readonly eventEditConnector = 'llm-tools-page-edit-connector';
    static readonly eventDeleteConnector = 'llm-tools-page-delete-connector';

    heading = 'Tools';
    description = 'The tools and agents that the assistant uses';

    // The namespace whose tools are shown, or none, for every tool, and the
    // name of its connector, when it's a connector's
    @property({ attribute: false })
    namespace?: string;

    @property({ attribute: false })
    connector?: string;

    // The templates of the servers which a connector can be added for,
    // which are in the menu which adds one, as well as a custom one
    @property({ attribute: false })
    templates: ConnectorTemplate[] = [];

    protected renderIcon(): unknown {
        return iconLoader(ToolKit32);
    }

    protected renderActions(): unknown {
        const agents = this.namespace === 'agent';
        return html`
            ${this.namespace && this.connector ? html`
                <cds-button kind="secondary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(ToolsPage.eventEditConnector, { detail: this.connector }))}>
                    Edit ${this.namespace} ${iconLoader(Edit16, { slot: 'icon' })}
                </cds-button>
                <cds-button kind="secondary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(ToolsPage.eventDeleteConnector, { detail: this.connector }))}>
                    Delete ${iconLoader(TrashCan16, { slot: 'icon' })}
                </cds-button>
            ` : nothing}
            ${agents ? html`
                <cds-button kind="primary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(ToolsPage.eventNewAgent))}>
                    New agent ${iconLoader(Add16, { slot: 'icon' })}
                </cds-button>
            ` : nothing}
            ${this.namespace ? nothing : html`
                <cds-menu-button label="Add connector" kind="primary" size="md">
                    <cds-menu size="md">
                        ${this.templates.map((template) => html`
                            <cds-menu-item label=${template.name}
                                @click=${() => this.dispatchEvent(new CustomEvent<ConnectorTemplate | undefined>(ToolsPage.eventAddConnector, { detail: template }))}>
                                ${connectorIcon(template) ? html`<img slot="render-icon" class="brand" src=${connectorIcon(template)} alt="">` : iconLoader(LogoMCP16, { slot: 'render-icon' })}
                            </cds-menu-item>
                        `)}
                        ${this.templates.length ? html`<cds-menu-item-divider></cds-menu-item-divider>` : nothing}
                        <cds-menu-item label="Custom connector"
                            @click=${() => this.dispatchEvent(new CustomEvent<ConnectorTemplate | undefined>(ToolsPage.eventAddConnector, { detail: undefined }))}>
                            ${iconLoader(LogoMCP16, { slot: 'render-icon' })}
                        </cds-menu-item>
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
