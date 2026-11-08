// An alert for each connector whose server isn't connected, with why, above
// the tools, so it's seen whichever namespace's tools are shown, until it's
// connected again, and one, with a Sign in button, for each whose server
// needs to be signed in to, which fires an event with its name, as it's
// clicked

import { LitElement, css, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { repeat } from 'lit/directives/repeat.js';
import { needsSignIn, type Connector } from '../schema/connector';

// Carbon components
import '@carbon/web-components/es/components/notification/index.js';

@customElement('llm-connector-alerts')
export class ConnectorAlerts extends LitElement {
    // The names of the events fired with the name of the connector, as its
    // Sign in, or Cancel, which forgets it, is clicked
    static readonly eventSignIn = 'llm-connector-alerts-sign-in';
    static readonly eventCancel = 'llm-connector-alerts-cancel';

    // The connectors, of which those which aren't connected, and have an
    // error, have alerts
    @property({ attribute: false })
    connectors: Connector[] = [];

    static styles = css`
        :host {
            display: block;
        }

        cds-inline-notification, cds-actionable-notification {
            max-width: none;
            margin: 0 0 1rem;
        }

        /* The notification's actions are side by side */
        .actions {
            display: flex;
            align-items: center;
            gap: 0.25rem;
        }
    `;

    render() {
        const signIn = this.connectors.filter((connector) => needsSignIn(connector));
        const down = this.connectors.filter((connector) => !connector.connected && connector.error && !needsSignIn(connector));
        return html`${repeat(signIn, (connector) => connector.name, (connector) => html`
            <cds-actionable-notification kind="warning" inline low-contrast hide-close-button .hasFocus=${false}
                title=${`${connector.name} needs you to sign in`} subtitle="Its server needs you to sign in, before its tools can be used">
                <div slot="action" class="actions">
                    <cds-actionable-notification-button kind="ghost" @click=${() => this.fire(ConnectorAlerts.eventSignIn, connector)}>Sign in</cds-actionable-notification-button>
                    <cds-actionable-notification-button kind="ghost" @click=${() => this.fire(ConnectorAlerts.eventCancel, connector)}>Cancel</cds-actionable-notification-button>
                </div>
            </cds-actionable-notification>
        `)}${repeat(down, (connector) => connector.name, (connector) => html`
            <cds-inline-notification kind="error" low-contrast hide-close-button
                title=${`${connector.name} isn't connected`} subtitle=${connector.error ?? ''}></cds-inline-notification>
        `)}`;
    }

    // fire fires the event, with the connector's name, as one of the
    // notification's actions is clicked
    private fire(name: string, connector: Connector) {
        this.dispatchEvent(new CustomEvent(name, { detail: connector.name, bubbles: true, composed: true }));
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-connector-alerts': ConnectorAlerts;
    }
}
