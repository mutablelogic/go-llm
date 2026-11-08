// A section of a form, which is hidden until it's disclosed: a line across
// the form, with the section's title on the left, and a button on the
// right, which shows, or hides, what's in it. It's closed each time it's
// made, such as when a panel's form is made again, as it opens.

import { LitElement, css, html } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import ChevronDown16 from '@carbon/icons/es/chevron--down/16.js';

@customElement('llm-disclosure')
export class Disclosure extends LitElement {
    // The section's title, which isn't title, so it isn't a tooltip of the
    // whole section
    @property()
    label = '';

    // Whether what's in it is shown
    @state()
    private open = false;

    static styles = css`
        :host {
            display: block;
        }

        .header {
            display: flex;
            align-items: center;
            gap: 1rem;
            padding-top: 0.5rem;
            border-top: 1px solid var(--cds-border-subtle-01);
        }

        .title {
            flex: 1;
            font-size: 0.875rem;
            font-weight: 600;
            line-height: 1.29;
            letter-spacing: 0.16px;
            color: var(--cds-text-primary);
        }

        button {
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
            padding: 0.5rem 0;
            border: none;
            background: none;
            font: inherit;
            font-size: 0.875rem;
            line-height: 1.29;
            letter-spacing: 0.16px;
            color: var(--cds-link-primary);
            cursor: pointer;
        }

        button:hover {
            color: var(--cds-link-primary-hover);
            text-decoration: underline;
        }

        button:focus-visible {
            outline: 2px solid var(--cds-focus);
            outline-offset: 2px;
        }

        button svg {
            fill: currentColor;
            transition: transform 110ms cubic-bezier(0.2, 0, 0.38, 0.9);
        }

        button[aria-expanded='true'] svg {
            transform: rotate(180deg);
        }

        .content {
            padding-top: 1rem;
        }

        .content[hidden] {
            display: none;
        }
    `;

    render() {
        return html`
            <div class="header">
                <span class="title">${this.label}</span>
                <button type="button" aria-expanded=${this.open ? 'true' : 'false'} aria-controls="content" @click=${this.toggle}>
                    ${this.open ? 'Hide' : 'Show'} ${iconLoader(ChevronDown16)}
                </button>
            </div>
            <div id="content" class="content" ?hidden=${!this.open}><slot></slot></div>
        `;
    }

    private toggle() {
        this.open = !this.open;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-disclosure': Disclosure;
    }
}
