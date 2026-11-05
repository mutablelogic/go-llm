// A panel on the right of the page, which shows an item, with a title, a
// subtitle and a body, which a subclass returns, over the page beneath the
// header, which is darkened. It's open while it has an item, and fires an
// event when it's closed, by its button, the escape key, or clicking the page
// beside it.

import { LitElement, css, html, nothing, type CSSResultGroup, type PropertyValues } from 'lit';
import { property } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Close16 from '@carbon/icons/es/close/16.js';

// Carbon components
import '@carbon/web-components/es/components/icon-button/index.js';

export abstract class Panel<T> extends LitElement {
    // The name of the event fired when the panel is closed
    static readonly eventClose = 'llm-panel-close';

    // Whether the panel is open, which is reflected, so it's styled
    @property({ type: Boolean, reflect: true })
    open = false;

    // The item which is shown, which is kept while the panel is closing
    private shown?: T;

    // What was focused when the panel opened, which is focused again when it
    // closes
    private returnFocus?: HTMLElement;

    // The elements which are made inert while the panel is open, as it's a
    // modal dialog, so focus, and clicks, stay in it
    private inerted: HTMLElement[] = [];

    static styles: CSSResultGroup = css`
        /* The panel covers the page beneath the header, which is hidden
           once the panel has closed */
        :host {
            position: fixed;
            inset: 3rem 0 0 0;
            z-index: 8000;
            visibility: hidden;
            pointer-events: none;
            transition: visibility 0s 110ms;
        }

        :host([open]) {
            visibility: visible;
            pointer-events: auto;
            transition: none;
        }

        /* The page beside the panel is darkened, and closes the panel when
           it's clicked */
        .backdrop {
            position: absolute;
            inset: 0;
            background: var(--cds-overlay, rgba(22, 22, 22, 0.5));
            opacity: 0;
            transition: opacity 110ms cubic-bezier(0.2, 0, 0.38, 0.9);
        }

        :host([open]) .backdrop {
            opacity: 1;
            transition: opacity 240ms cubic-bezier(0, 0, 0.38, 0.9);
        }

        .panel {
            position: absolute;
            top: 0;
            right: 0;
            bottom: 0;
            display: flex;
            flex-direction: column;
            width: min(28rem, 100vw);
            background: var(--cds-layer-01);
            box-shadow: -2px 0 6px rgba(0, 0, 0, 0.2);
            transform: translateX(100%);
            transition: transform 110ms cubic-bezier(0.2, 0, 0.38, 0.9);
        }

        /* The panel is focused when it opens, but isn't outlined, as it isn't
           clicked, or tabbed to */
        .panel:focus {
            outline: none;
        }

        :host([open]) .panel {
            transform: none;
            transition: transform 240ms cubic-bezier(0, 0, 0.38, 0.9);
        }

        header {
            display: flex;
            align-items: flex-start;
            gap: 0.5rem;
            padding: 1rem 0.5rem 1rem 1rem;
            border-bottom: 1px solid var(--cds-border-subtle-01);
        }

        .title {
            flex: 1;
            min-width: 0;
        }

        h2 {
            margin: 0;
            font-size: var(--cds-heading-03-font-size, 1.25rem);
            font-weight: var(--cds-heading-03-font-weight, 400);
            line-height: var(--cds-heading-03-line-height, 1.4);
            overflow-wrap: anywhere;
        }

        .subtitle {
            margin-top: 0.25rem;
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
            overflow-wrap: anywhere;
        }

        .body {
            flex: 1;
            overflow-y: auto;
            padding: 1rem;
        }

        p {
            margin: 0 0 1.5rem;
            font-size: 0.875rem;
        }

        h3 {
            margin: 1.5rem 0 0.5rem;
            font-size: var(--cds-heading-01-font-size, 0.875rem);
            font-weight: var(--cds-heading-01-font-weight, 600);
        }

        h3:first-child {
            margin-top: 0;
        }

        dl {
            display: grid;
            grid-template-columns: minmax(6rem, max-content) minmax(0, 1fr);
            gap: 0;
            margin: 0;
            font-size: 0.875rem;
        }

        dt,
        dd {
            margin: 0;
            padding: 0.5rem 0;
            border-bottom: 1px solid var(--cds-border-subtle-01);
        }

        dt {
            padding-right: 1rem;
            color: var(--cds-text-secondary);
        }

        dd {
            overflow-wrap: anywhere;
        }

        code {
            font-family: var(--cds-code-01-font-family, 'IBM Plex Mono', monospace);
            font-size: 0.75rem;
            white-space: pre-wrap;
        }
    `;

    // current returns the item which is shown, which is undefined when the
    // panel is closed
    protected abstract current(): T | undefined;

    // renderTitle, renderSubtitle and renderBody return the item's title, the
    // subtitle beneath it, and the body of the panel
    protected abstract renderTitle(item: T): unknown;
    protected abstract renderSubtitle(item: T): unknown;
    protected abstract renderBody(item: T): unknown;

    connectedCallback() {
        super.connectedCallback();
        document.addEventListener('keydown', this.handleKeydown);
    }

    disconnectedCallback() {
        document.removeEventListener('keydown', this.handleKeydown);
        this.restoreBackground();
        super.disconnectedCallback();
    }

    // Whether the panel is opening for the first time, when it's rendered
    // closed, and then opened, once it's rendered, so it slides in, as it
    // isn't rendered until it has an item, and would otherwise be rendered
    // open, without its transition
    private opening = false;

    willUpdate(_changed: PropertyValues<this>) {
        const item = this.current();
        if (item !== undefined && this.shown === undefined) {
            this.shown = item;
            this.open = false;
            this.opening = true;
            return;
        } else if (this.opening) {
            this.shown = item ?? this.shown;
            return;
        }
        this.open = item !== undefined;
        this.shown = item ?? this.shown;
    }

    // updated makes the page beside the panel inert, and focuses the panel,
    // when it opens, as it's a modal dialog, and restores the
    // page, and focuses what was focused before, when it closes
    updated(changed: PropertyValues<this>) {
        // The panel which is opening for the first time is rendered closed,
        // and its style is computed, so it's opened with its transition
        if (this.opening) {
            this.opening = false;
            this.renderRoot.querySelector('.panel')?.getBoundingClientRect();
            this.requestUpdate();
            return;
        }
        if (!changed.has('open') || changed.get('open') === undefined && !this.open) {
            return;
        }
        if (this.open) {
            this.returnFocus = deepActiveElement();
            this.inertBackground();
            // The panel is focused, rather than its close button, whose
            // tooltip is shown when it's focused, so it's shown when it's
            // tabbed to, rather than whenever the panel opens
            this.renderRoot.querySelector<HTMLElement>('.panel')?.focus();
        } else {
            this.restoreBackground();
            this.returnFocus?.focus();
            this.returnFocus = undefined;
        }
    }

    // inertBackground makes the elements beside the panel, and beside each
    // of the elements it's in, up to the body, inert, which aren't already,
    // so only the panel can be focused, or clicked, but not the regions which
    // announce notifications, which are read, rather than used
    private inertBackground() {
        this.restoreBackground();
        for (let element: HTMLElement = this; element.parentElement && element !== document.body; element = element.parentElement) {
            for (const sibling of Array.from(element.parentElement.children)) {
                if (sibling !== element && sibling instanceof HTMLElement && !sibling.inert && !sibling.hasAttribute('aria-live')) {
                    sibling.inert = true;
                    this.inerted.push(sibling);
                }
            }
        }
    }

    // restoreBackground makes the elements which were made inert no longer
    // inert
    private restoreBackground() {
        for (const element of this.inerted) {
            element.inert = false;
        }
        this.inerted = [];
    }

    render() {
        const item = this.shown;
        if (item === undefined) {
            return nothing;
        }
        return html`
            <div class="backdrop" @click=${this.close}></div>
            <div class="panel" role="dialog" aria-modal="true" aria-labelledby="title" tabindex="-1">
                <header>
                    <div class="title">
                        <h2 id="title">${this.renderTitle(item)}</h2>
                        <div class="subtitle">${this.renderSubtitle(item)}</div>
                    </div>
                    <cds-icon-button kind="ghost" size="md" align="left" @click=${this.close}>
                        ${iconLoader(Close16, { slot: 'icon' })}
                        <span slot="tooltip-content">Close</span>
                    </cds-icon-button>
                </header>
                <div class="body">${this.renderBody(item)}</div>
            </div>
        `;
    }

    private close() {
        this.dispatchEvent(new CustomEvent(Panel.eventClose));
    }

    // handleKeydown closes the panel when the escape key is pressed, while
    // it's open
    private readonly handleKeydown = (event: KeyboardEvent) => {
        if (event.key === 'Escape' && this.open) {
            this.close();
        }
    };
}

// deepActiveElement returns the element which is focused, in the shadow roots
// of the elements which are focused
function deepActiveElement(): HTMLElement | undefined {
    let element = document.activeElement;
    while (element?.shadowRoot?.activeElement) {
        element = element.shadowRoot.activeElement;
    }
    return element instanceof HTMLElement ? element : undefined;
}

// formatValue returns a value of metadata, which is text, or JSON when it's an
// object or an array
export function formatValue(value: unknown): unknown {
    if (value === null || value === undefined) {
        return '';
    } else if (typeof value === 'object') {
        return html`<code>${JSON.stringify(value, null, 2)}</code>`;
    }
    return String(value);
}
