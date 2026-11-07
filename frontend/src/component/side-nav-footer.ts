// The footer of the side navigation, which shows the version of the server,
// as it was when the page was loaded, which is the page's, with a warning,
// when the server has another version now, such as when it was updated as
// it was down, which reloads the page, and links to the API's documentation
// when the server serves it, and to the project's repository on GitHub

import { LitElement, css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Api16 from '@carbon/icons/es/api/16.js';
import LogoGithub16 from '@carbon/icons/es/logo--github/16.js';
import WarningAltFilled16 from '@carbon/icons/es/warning--alt--filled/16.js';
import type { Config } from '../schema/config';

// Carbon components
import '@carbon/web-components/es/components/icon-button/index.js';

// repository is the project's repository, which the footer links to
const repository = 'https://github.com/mutablelogic/go-llm';

@customElement('llm-side-nav-footer')
export class SideNavFooter extends LitElement {
    // The server's configuration, which is undefined until it's read
    @property({ attribute: false })
    config?: Config;

    // The server's configuration as it is now, which is read again when the
    // server can be reached again, and is undefined until it's read
    @property({ attribute: false })
    latest?: Config;

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

        .links {
            display: flex;
            margin-right: -0.5rem;
        }

        .version {
            display: flex;
            align-items: center;
            gap: 0.25rem;
        }

        .version cds-icon-button {
            margin: 0;
        }

        .warning {
            color: var(--cds-support-warning, #f1c21b);
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
            <span class="version">
                <span>${this.config.version ?? ''}</span>
                ${this.outdated() ? html`
                    <cds-icon-button kind="ghost" size="sm" align="top-start" @click=${() => location.reload()}>
                        <span slot="icon" class="icon warning">${iconLoader(WarningAltFilled16)}<span class="visually-hidden">The server was updated</span></span>
                        <span slot="tooltip-content">The server is now ${this.latest?.version || 'another version'}. Reload the page to use it.</span>
                    </cds-icon-button>
                ` : nothing}
            </span>
            <span class="links">
                ${this.config.openapi
                    ? html`
                        <cds-icon-button kind="ghost" size="sm" align="top-end" href=${this.config.openapi} target="_blank" rel="noopener">
                            <span slot="icon" class="icon">${iconLoader(Api16)}<span class="visually-hidden">OpenAPI documentation</span></span>
                            <span slot="tooltip-content">OpenAPI documentation</span>
                        </cds-icon-button>
                    `
                    : nothing}
                <cds-icon-button kind="ghost" size="sm" align="top-end" href=${repository} target="_blank" rel="noopener">
                    <span slot="icon" class="icon">${iconLoader(LogoGithub16)}<span class="visually-hidden">go-llm on GitHub</span></span>
                    <span slot="tooltip-content">go-llm on GitHub</span>
                </cds-icon-button>
            </span>
        `;
    }

    // outdated returns whether the server has another version now than the
    // page, which was loaded from it
    private outdated(): boolean {
        return !!this.config && !!this.latest && (this.latest.version ?? '') !== (this.config.version ?? '');
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-side-nav-footer': SideNavFooter;
    }
}
