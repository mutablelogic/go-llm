// An alert for each connector whose server isn't connected, with why, above
// the tools, so it's seen whichever namespace's tools are shown, until it's
// connected again

import { LitElement, css, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { repeat } from 'lit/directives/repeat.js';
import type { Connector } from '../schema/connector';

// Carbon components
import '@carbon/web-components/es/components/notification/index.js';

@customElement('llm-connector-alerts')
export class ConnectorAlerts extends LitElement {
    // The connectors, of which those which aren't connected, and have an
    // error, have alerts
    @property({ attribute: false })
    connectors: Connector[] = [];

    static styles = css`
        :host {
            display: block;
        }

        cds-inline-notification {
            max-width: none;
            margin: 0 0 1rem;
        }
    `;

    render() {
        const down = this.connectors.filter((connector) => !connector.connected && connector.error);
        return html`${repeat(down, (connector) => connector.name, (connector) => html`
            <cds-inline-notification kind="error" low-contrast hide-close-button
                title=${`${connector.name} isn't connected`} subtitle=${connector.error ?? ''}></cds-inline-notification>
        `)}`;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-connector-alerts': ConnectorAlerts;
    }
}
