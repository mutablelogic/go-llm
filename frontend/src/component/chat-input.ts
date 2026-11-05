// A box for a message to a chat session, with a button which sends it, which
// grows with its text. Enter sends the message, and shift and enter starts a
// new line. It fires an event with the message, and is busy while it's
// being replied to, when the next can be typed, but not sent, so the box
// keeps its focus, and an event as the message is typed, so others are told
// someone is typing.

import { LitElement, css, html } from 'lit';
import { customElement, property, query } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Send16 from '@carbon/icons/es/send/16.js';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';

@customElement('llm-chat-input')
export class ChatInput extends LitElement {
    // The name of the event fired when a message is sent, whose detail is its
    // text
    static readonly eventSend = 'llm-chat-send';

    // The name of the event fired as the message is typed
    static readonly eventTyping = 'llm-chat-typing';

    // The most lines the box grows to, before it scrolls
    static readonly maxLines = 8;

    // Whether the box is disabled
    @property({ type: Boolean, reflect: true })
    disabled = false;

    // Whether a message is being replied to, when another can be typed, but
    // not sent, so the box keeps its focus
    @property({ type: Boolean, reflect: true })
    busy = false;

    // The placeholder, shown while the box is empty
    @property()
    placeholder = 'Ask a question, and press enter to send it';

    @query('textarea')
    private textarea!: HTMLTextAreaElement;

    static styles = css`
        :host {
            display: flex;
            align-items: flex-end;
            gap: 0.5rem;
            padding: 1rem 2rem;
            border-top: 1px solid var(--cds-border-subtle-01);
            background: var(--cds-background);
        }

        textarea {
            flex: 1;
            box-sizing: border-box;
            min-height: 2.5rem;
            padding: 0.6875rem 1rem;
            border: none;
            border-bottom: 1px solid var(--cds-border-strong-01);
            outline: 2px solid transparent;
            outline-offset: -2px;
            background: var(--cds-field-01, #f4f4f4);
            color: var(--cds-text-primary);
            font-family: inherit;
            font-size: var(--cds-body-compact-01-font-size, 0.875rem);
            line-height: 1.125rem;
            resize: none;
        }

        textarea:focus {
            outline-color: var(--cds-focus, #0f62fe);
        }

        textarea::placeholder {
            color: var(--cds-text-placeholder);
        }

        textarea:disabled {
            color: var(--cds-text-disabled);
            cursor: not-allowed;
        }
    `;

    render() {
        return html`
            <textarea rows="1" aria-label="Message" placeholder=${this.placeholder} ?disabled=${this.disabled}
                @input=${this.handleInput} @keydown=${this.handleKeydown}></textarea>
            <cds-button kind="primary" size="md" ?disabled=${this.disabled || this.busy} @click=${this.send}>
                Send ${iconLoader(Send16, { slot: 'icon' })}
            </cds-button>
        `;
    }

    // focus focuses the box, once it's rendered, as it can't be focused
    // while it's disabled
    focus() {
        void this.updateComplete.then(() => this.textarea?.focus());
    }

    // restore puts the message back in the box, such as when it couldn't be
    // sent, unless another has been started
    restore(text: string) {
        if (this.textarea && this.textarea.value.trim() === '') {
            this.textarea.value = text;
            this.resize();
        }
    }

    // handleInput grows the box to its text, and fires the event that
    // someone is typing
    private handleInput() {
        this.resize();
        this.dispatchEvent(new CustomEvent(ChatInput.eventTyping));
    }

    // send fires the event with the message, unless it's empty, and empties
    // the box
    private send() {
        const text = this.textarea.value.trim();
        if (this.disabled || this.busy || !text) {
            return;
        }
        this.dispatchEvent(new CustomEvent(ChatInput.eventSend, { detail: text }));
        this.textarea.value = '';
        this.resize();
    }

    // handleKeydown sends the message when enter is pressed, unless it's with
    // shift, which starts a new line, or a character is being composed
    private handleKeydown(event: KeyboardEvent) {
        if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
            event.preventDefault();
            this.send();
        }
    }

    // resize grows the box to its text, up to the most lines
    private resize() {
        const textarea = this.textarea;
        const line = parseFloat(getComputedStyle(textarea).lineHeight);
        textarea.style.height = 'auto';
        textarea.style.height = `${Math.min(textarea.scrollHeight, line * ChatInput.maxLines + 22)}px`;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-chat-input': ChatInput;
    }
}
