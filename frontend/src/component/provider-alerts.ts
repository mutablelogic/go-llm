// An alert for each provider which can't be reached, with why, above the
// models, so it's seen whichever provider's models are shown

import { LitElement, css, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { repeat } from 'lit/directives/repeat.js';
import type { Provider } from '../schema/provider';

// Carbon components
import '@carbon/web-components/es/components/notification/index.js';

@customElement('llm-provider-alerts')
export class ProviderAlerts extends LitElement {
    // The providers, of which those which can't be reached have alerts
    @property({ attribute: false })
    providers: Provider[] = [];

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
        const down = this.providers.filter((provider) => provider.error);
        return html`${repeat(down, (provider) => provider.name, (provider) => html`
            <cds-inline-notification kind="error" low-contrast hide-close-button
                title=${`${provider.name} can't be reached`} subtitle=${provider.error ?? ''}></cds-inline-notification>
        `)}`;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-provider-alerts': ProviderAlerts;
    }
}
