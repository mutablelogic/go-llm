// A page, which has a heading, with an icon, and a description beneath it,
// and controls beside it, in the slot "heading", and actions at its end, which
// a page returns, and its content, which is slotted, so the views on it are in the document,
// where controllers find them. The heading is the part "heading", so a page
// can be styled, and its description's colour is the property
// --llm-heading-secondary.

import { LitElement, css, html, nothing, type CSSResultGroup } from 'lit';
import { property } from 'lit/decorators.js';

export class Page extends LitElement {
    // The page's heading, and its description, which is shown beneath it
    @property()
    heading = '';

    // The description is text, or a template, such as with an icon
    @property({ attribute: false })
    description: unknown = '';

    static styles: CSSResultGroup = css`
        :host {
            display: block;
        }

        :host([hidden]) {
            display: none;
        }

        header {
            display: flex;
            align-items: center;
            gap: 1rem;
            margin: 0 0 1.5rem;
        }

        .icon {
            display: flex;
            flex: none;
        }

        .icon svg {
            fill: currentColor;
        }

        /* The actions are focused in the heading's colour, as the theme
           classes don't reach into the page */
        .actions {
            --cds-focus: #ffffff;
            display: flex;
            flex: none;
            gap: 1px;
            margin-left: auto;
        }

        /* The controls beside the heading fill the rest of it */
        ::slotted([slot='heading']) {
            flex: 1;
            min-width: 0;
        }

        h1 {
            font-size: var(--cds-heading-04-font-size, 1.75rem);
            font-weight: var(--cds-heading-04-font-weight, 400);
            line-height: var(--cds-heading-04-line-height, 1.28572);
            margin: 0;
        }

        p {
            margin: 0.125rem 0 0;
            color: var(--llm-heading-secondary, var(--cds-text-secondary));
            font-size: var(--cds-body-compact-01-font-size, 0.875rem);
            line-height: var(--cds-body-compact-01-line-height, 1.28572);
        }

        /* In a description, an icon, such as that a conversation's memory is
           nearly full, and a small button beside it, such as to compact it,
           are in its colour, so they're seen, but aren't loud */
        p svg {
            vertical-align: -3px;
            fill: currentColor;
        }

        p .nearly-full {
            margin-left: 0.25rem;
        }

        p button {
            margin: 0 0 0 0.375rem;
            padding: 0;
            border: none;
            background: none;
            color: inherit;
            line-height: 1;
            cursor: pointer;
            opacity: 0.8;
        }

        p button:hover, p button:focus-visible {
            opacity: 1;
        }
    `;

    render() {
        // The heading is shown when there's a heading, or controls beside it
        const icon = this.renderIcon();
        const actions = this.renderActions();
        const controls = this.querySelector(':scope > [slot="heading"]') !== null;
        return html`
            ${this.heading || controls ? html`
                <header part="heading">
                    ${icon ? html`<span class="icon">${icon}</span>` : nothing}
                    ${this.heading ? html`
                        <div>
                            <h1>${this.heading}</h1>
                            ${this.description ? html`<p>${this.description}</p>` : nothing}
                        </div>
                    ` : nothing}
                    <slot name="heading"></slot>
                    ${actions ? html`<div class="actions">${actions}</div>` : nothing}
                </header>
            ` : nothing}
            <slot></slot>
        `;
    }

    // renderIcon returns the icon beside the heading, or nothing
    protected renderIcon(): unknown {
        return nothing;
    }

    // renderActions returns the actions at the end of the heading, such as
    // buttons, or nothing
    protected renderActions(): unknown {
        return nothing;
    }
}
