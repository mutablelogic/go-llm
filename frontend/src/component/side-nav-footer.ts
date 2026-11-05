// The footer of the side navigation, which shows the version of the server,
// and links to the API's documentation when the server serves it

import { LitElement, css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Api16 from '@carbon/icons/es/api/16.js';
import type { Config } from '../schema/config';

// Carbon components
import '@carbon/web-components/es/components/icon-button/index.js';

@customElement('llm-side-nav-footer')
export class SideNavFooter extends LitElement {
    // The server's configuration, which is undefined until it's read
    @property({ attribute: false })
    config?: Config;

    static styles = css`
        :host {
            position: absolute;
            right: 0;
            bottom: 0;
            left: 0;
            display: flex;
            align-items: center;
            justify-content: space-between;
            min-height: 3rem;
            padding: 0 1rem;
            border-top: 1px solid var(--cds-border-subtle);
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
            letter-spacing: 0.32px;
        }

        cds-icon-button {
            margin-right: -0.5rem;
        }

        .icon {
            display: flex;
        }

        /* The name of the link, for screen readers, as Carbon only names an
           icon button which isn't a link */
        .visually-hidden {
            position: absolute;
            width: 1px;
            height: 1px;
            overflow: hidden;
            clip: rect(0 0 0 0);
            white-space: nowrap;
        }
    `;

    render() {
        if (!this.config) {
            return nothing;
        }
        return html`
            <span>${this.config.version ?? ''}</span>
            ${this.config.openapi
                ? html`
                    <cds-icon-button kind="ghost" size="sm" align="top-end" href=${this.config.openapi} target="_blank" rel="noopener">
                        <span slot="icon" class="icon">${iconLoader(Api16)}<span class="visually-hidden">OpenAPI documentation</span></span>
                        <span slot="tooltip-content">OpenAPI documentation</span>
                    </cds-icon-button>
                `
                : nothing}
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-side-nav-footer': SideNavFooter;
    }
}
