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
    // The name of the event fired with the name of the connector, as its
    // Sign in is clicked
    static readonly eventSignIn = 'llm-connector-alerts-sign-in';

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
    `;

    render() {
        const signIn = this.connectors.filter((connector) => needsSignIn(connector));
        const down = this.connectors.filter((connector) => !connector.connected && connector.error && !needsSignIn(connector));
        return html`${repeat(signIn, (connector) => connector.name, (connector) => html`
            <cds-actionable-notification kind="warning" inline low-contrast hide-close-button .hasFocus=${false}
                action-button-label="Sign in" @click=${(event: Event) => this.handleClick(event, connector)}
                title=${`${connector.name} needs you to sign in`} subtitle="Its server needs you to sign in, before its tools can be used"></cds-actionable-notification>
        `)}${repeat(down, (connector) => connector.name, (connector) => html`
            <cds-inline-notification kind="error" low-contrast hide-close-button
                title=${`${connector.name} isn't connected`} subtitle=${connector.error ?? ''}></cds-inline-notification>
        `)}`;
    }

    // handleClick fires the event, once the notification's action button is
    // clicked, rather than anywhere else in it
    private handleClick(event: Event, connector: Connector) {
        if (event.composedPath().some((target) => target instanceof Element && target.localName === 'cds-actionable-notification-button')) {
            this.dispatchEvent(new CustomEvent(ConnectorAlerts.eventSignIn, { detail: connector.name, bubbles: true, composed: true }));
        }
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-connector-alerts': ConnectorAlerts;
    }
}
