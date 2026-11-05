// A chat bubble, which shows a piece of a message, which is the user's, on
// the right, the assistant's, on the left, or the assistant's thinking, which
// is collapsed, beneath a label, and expanded when it's clicked, or while it's
// live, as it's being generated. Its content is slotted. The assistant's
// bubbles have actions which copy their text, and fire an event so their
// message's details are shown, which they don't have while they're live.

import { LitElement, css, html, nothing, type PropertyValues } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Checkmark16 from '@carbon/icons/es/checkmark/16.js';
import Copy16 from '@carbon/icons/es/copy/16.js';
import Information16 from '@carbon/icons/es/information/16.js';

// ChatKind is who a bubble is from, or what it is, which isn't its role, as
// that's an ARIA attribute
export type ChatKind = 'user' | 'assistant' | 'thinking';

@customElement('llm-chat-bubble')
export class ChatBubble extends LitElement {
    // The name of the event fired when the details action is clicked
    static readonly eventInfo = 'llm-chat-bubble-info';

    // How long the copy action shows it's copied, in milliseconds
    static readonly copiedFor = 1500;

    // Who the bubble is from, or what it is, which is reflected, so it's
    // styled
    @property({ reflect: true })
    kind: ChatKind = 'assistant';

    // The label of a thinking bubble, which is shown while it's collapsed
    @property()
    label = 'Thinking';

    // Whether a thinking bubble is expanded
    @property({ type: Boolean, reflect: true })
    open = false;

    // Whether the bubble is live, as it's being generated. A thinking bubble
    // is expanded while it's live, and collapsed when it's no longer live,
    // unless it's expanded or collapsed in between.
    @property({ type: Boolean, reflect: true })
    live = false;

    // Whether a thinking bubble has finished collapsing, when its thinking no
    // longer widens it, which isn't while it's collapsing, as it'd be narrow
    @state()
    private collapsed = true;

    // Whether the bubble's message is selected, as its details are shown
    @property({ type: Boolean, reflect: true })
    selected = false;

    // The text which is copied, such as Markdown which is shown as HTML, or
    // the bubble's text, when it's empty
    @property({ attribute: false })
    copyText = '';

    // Whether the text has just been copied
    @state()
    private copied = false;

    private copiedTimer?: number;

    static styles = css`
        :host {
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            max-width: 100%;
        }

        :host([kind='user']) {
            align-items: flex-end;
        }

        .bubble {
            box-sizing: border-box;
            max-width: min(40rem, 85%);
            padding: 0.75rem 1rem 0.5rem;
            border-radius: 1rem;
            font-size: var(--cds-body-01-font-size, 0.875rem);
            line-height: var(--cds-body-01-line-height, 1.42857);
            overflow-wrap: anywhere;
        }

        /* The user's bubbles are IBM blue, with their corner beneath them on
           the right, and the assistant's are on a layer, with their corner on
           the left */
        :host([kind='user']) .bubble {
            padding-bottom: 0.75rem;
            background: var(--cds-button-primary, #0f62fe);
            color: var(--cds-text-on-color, #ffffff);
            border-bottom-right-radius: 0.25rem;
        }

        :host([kind='assistant']) .bubble {
            background: var(--cds-layer-01, #f4f4f4);
            color: var(--cds-text-primary, #161616);
            border-bottom-left-radius: 0.25rem;
        }

        /* Thinking is quieter, with a dashed border, and collapsed */
        :host([kind='thinking']) .bubble {
            padding: 0.25rem 0.5rem 0.25rem 1rem;
            border: 1px dashed var(--cds-border-strong-01, #8d8d8d);
            color: var(--cds-text-secondary, #525252);
            font-style: italic;
        }

        /* A bubble whose message's details are shown is outlined */
        :host([selected]) .bubble {
            box-shadow: 0 0 0 2px var(--cds-focus, #0f62fe);
        }

        :host([selected][kind='user']) .bubble {
            box-shadow: 0 0 0 2px var(--cds-background, #ffffff), 0 0 0 4px var(--cds-focus, #0f62fe);
        }

        .header {
            display: flex;
            align-items: center;
            gap: 1rem;
            min-height: 1.5rem;
        }

        .toggle {
            display: flex;
            flex: 1;
            align-items: center;
            gap: 0.5rem;
            padding: 0;
            border: none;
            background: none;
            color: inherit;
            font: inherit;
            font-style: normal;
            cursor: pointer;
        }

        .toggle::before {
            content: '';
            border: 0.3rem solid transparent;
            border-left-color: currentColor;
            transform-origin: 25% 50%;
            transition: transform 110ms;
        }

        :host([open]) .toggle::before {
            transform: rotate(90deg);
        }

        /* The thinking expands and collapses, by its row's height */
        .thinking {
            display: grid;
            grid-template-rows: 0fr;
            transition: grid-template-rows 240ms cubic-bezier(0.2, 0, 0.38, 0.9);
        }

        :host([open]) .thinking {
            grid-template-rows: 1fr;
        }

        /* The thinking doesn't widen the bubble once it's collapsed */
        .thinking.collapsed {
            contain: inline-size;
        }

        .thinking > div {
            overflow: hidden;
        }

        .thinking > div > slot {
            display: block;
            padding: 0.25rem 0.5rem 0.5rem 0;
        }

        /* The actions are at the end of the bubble, and are quieter until
           the bubble is hovered, or they're focused */
        .actions {
            display: flex;
            justify-content: flex-end;
            gap: 0.125rem;
            margin: 0.25rem -0.5rem 0 0;
            opacity: 0.6;
            transition: opacity 110ms;
        }

        .header .actions {
            margin: 0;
        }

        :host(:hover) .actions,
        .actions:focus-within {
            opacity: 1;
        }

        .actions button {
            display: flex;
            align-items: center;
            justify-content: center;
            width: 1.5rem;
            height: 1.5rem;
            padding: 0;
            border: none;
            border-radius: 0.25rem;
            background: none;
            color: inherit;
            cursor: pointer;
        }

        .actions button:hover {
            background: rgba(141, 141, 141, 0.2);
        }

        .actions button:focus-visible {
            outline: 2px solid var(--cds-focus, #0f62fe);
            outline-offset: -2px;
        }

        .actions svg {
            fill: currentColor;
        }
    `;

    render() {
        if (this.kind === 'thinking') {
            return html`<div class="bubble">${this.renderThinking()}</div>`;
        }
        return html`<div class="bubble"><slot></slot>${this.renderActions()}</div>`;
    }

    // willUpdate expands the bubble when it becomes live, and collapses it
    // when it's no longer live
    willUpdate(changed: PropertyValues<this>) {
        if (changed.has('live') && (this.live || changed.get('live') !== undefined)) {
            this.open = this.live;
        }
        if (this.open) {
            this.collapsed = false;
        }
    }

    disconnectedCallback() {
        window.clearTimeout(this.copiedTimer);
        super.disconnectedCallback();
    }

    // renderThinking returns the label, which expands and collapses the
    // thinking, with the actions, and the thinking, which is hidden while
    // it's collapsed
    private renderThinking() {
        return html`
            <div class="header">
                <button class="toggle" type="button" aria-expanded=${String(this.open)} @click=${() => { this.open = !this.open; }}>${this.live ? `${this.label}…` : this.label}</button>
                ${this.renderActions()}
            </div>
            <div class="thinking ${this.collapsed ? 'collapsed' : ''}" ?inert=${!this.open}
                @transitionend=${() => { this.collapsed = !this.open; }}><div><slot></slot></div></div>
        `;
    }

    // renderActions returns the actions, which copy the text, and show the
    // message's details, unless the bubble is live, or the user's
    private renderActions() {
        if (this.live || this.kind === 'user') {
            return nothing;
        }
        const copy = this.copied ? 'Copied' : 'Copy';
        return html`
            <div class="actions">
                <button type="button" title=${copy} aria-label=${copy} @click=${this.copy}>
                    ${iconLoader(this.copied ? Checkmark16 : Copy16)}
                </button>
                <button type="button" title="Details" aria-label="Details"
                    @click=${() => this.dispatchEvent(new CustomEvent(ChatBubble.eventInfo, { bubbles: true, composed: true }))}>
                    ${iconLoader(Information16)}
                </button>
            </div>
        `;
    }

    // copy copies the bubble's text to the clipboard, and shows it's copied
    private async copy() {
        const slot = this.renderRoot.querySelector<HTMLSlotElement>('slot:not([name])');
        const text = this.copyText || (slot?.assignedNodes({ flatten: true }) ?? []).map((node) => node.textContent ?? '').join('').trim();
        await navigator.clipboard.writeText(text);
        this.copied = true;
        window.clearTimeout(this.copiedTimer);
        this.copiedTimer = window.setTimeout(() => { this.copied = false; }, ChatBubble.copiedFor);
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-chat-bubble': ChatBubble;
    }
}
