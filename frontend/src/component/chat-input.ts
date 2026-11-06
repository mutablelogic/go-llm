// A box for a message to a chat session, with a button which sends it, which
// grows with its text. Enter sends the message, and shift and enter starts a
// new line. It fires an event with the message, and is busy while it's
// being replied to, when the next can be typed, but not sent, so the box
// keeps its focus, and an event as the message is typed, so others are told
// someone is typing. Its add button, to the left, fires an event to choose
// resources to attach, which are shown above the box, each of which can be
// removed, and files which are dropped on it, or pasted, fire an event to
// upload them, and attach them. A message with attachments can be sent
// without text.

import { LitElement, css, html, nothing } from 'lit';
import { customElement, property, query } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Send16 from '@carbon/icons/es/send/16.js';
import Add16 from '@carbon/icons/es/add/16.js';
import Close16 from '@carbon/icons/es/close/16.js';
import { hasFiles } from './resource-panel';
import { resourceName, type Resource } from '../schema/resource';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/icon-button/index.js';

@customElement('llm-chat-input')
export class ChatInput extends LitElement {
    // The name of the event fired when a message is sent, whose detail is its
    // text
    static readonly eventSend = 'llm-chat-send';

    // The name of the event fired as the message is typed
    static readonly eventTyping = 'llm-chat-typing';

    // The names of the events fired to choose resources to attach, to remove
    // an attachment, whose detail is its resource, and to upload files, and
    // attach them, whose detail is them
    static readonly eventAttach = 'llm-chat-attach';
    static readonly eventDetach = 'llm-chat-detach';
    static readonly eventUpload = 'llm-chat-upload';

    // The most lines the box grows to, before it scrolls
    static readonly maxLines = 8;

    // Whether the box is disabled
    @property({ type: Boolean, reflect: true })
    disabled = false;

    // Whether a message is being replied to, when another can be typed, but
    // not sent, so the box keeps its focus
    @property({ type: Boolean, reflect: true })
    busy = false;

    // The resources which are attached to the message
    @property({ attribute: false })
    attachments: Resource[] = [];

    // Whether files are dragged over the box
    @property({ type: Boolean, reflect: true })
    dragging = false;

    // The placeholder, shown while the box is empty
    @property()
    placeholder = 'Ask a question, and press enter to send it';

    @query('textarea')
    private textarea!: HTMLTextAreaElement;

    static styles = css`
        :host {
            display: flex;
            flex-direction: column;
            gap: 0.5rem;
            padding: 1rem 2rem;
            border-top: 1px solid var(--cds-border-subtle-01);
            background: var(--cds-background);
        }

        :host([dragging]) {
            outline: 2px dashed var(--cds-focus, #0f62fe);
            outline-offset: -4px;
            background: var(--cds-layer-hover-01);
        }

        .row {
            display: flex;
            align-items: flex-end;
            gap: 0.5rem;
        }

        /* The attachments, above the box, each with its button which
           removes it */
        .attachments {
            display: flex;
            flex-wrap: wrap;
            gap: 0.5rem;
        }

        .chip {
            display: inline-flex;
            align-items: center;
            gap: 0.25rem;
            max-width: 16rem;
            padding: 0 0.25rem 0 0.75rem;
            border-radius: 1rem;
            background: var(--cds-layer-accent-01, #e0e0e0);
            font-size: 0.75rem;
            line-height: 1.5rem;
        }

        .chip span {
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        .chip button {
            display: inline-flex;
            padding: 0.125rem;
            border: none;
            border-radius: 50%;
            background: none;
            color: inherit;
            cursor: pointer;
        }

        .chip button:hover {
            background: var(--cds-layer-accent-hover-01, #d1d1d1);
        }

        .chip button:focus-visible {
            outline: 2px solid var(--cds-focus, #0f62fe);
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

    connectedCallback() {
        super.connectedCallback();
        this.addEventListener('dragenter', this.handleDrag);
        this.addEventListener('dragover', this.handleDrag);
        this.addEventListener('dragleave', this.handleDragLeave);
        this.addEventListener('drop', this.handleDrop);
    }

    disconnectedCallback() {
        this.removeEventListener('dragenter', this.handleDrag);
        this.removeEventListener('dragover', this.handleDrag);
        this.removeEventListener('dragleave', this.handleDragLeave);
        this.removeEventListener('drop', this.handleDrop);
        super.disconnectedCallback();
    }

    render() {
        return html`
            ${this.attachments.length > 0 ? html`<div class="attachments">${this.attachments.map((resource) => html`
                <span class="chip" title=${resource.type ?? ''}>
                    <span>${resourceName(resource)}</span>
                    <button aria-label="Remove ${resourceName(resource)}" @click=${() => this.fire(ChatInput.eventDetach, resource)}>
                        ${iconLoader(Close16)}
                    </button>
                </span>`)}</div>` : nothing}
            <div class="row">
                <cds-icon-button kind="ghost" size="md" align="top-left" ?disabled=${this.disabled}
                    @click=${() => this.fire(ChatInput.eventAttach)}>
                    ${iconLoader(Add16, { slot: 'icon' })}
                    <span slot="tooltip-content">Attach files</span>
                </cds-icon-button>
                <textarea rows="1" aria-label="Message" placeholder=${this.placeholder} ?disabled=${this.disabled}
                    @input=${this.handleInput} @keydown=${this.handleKeydown} @paste=${this.handlePaste}></textarea>
                <cds-button kind="primary" size="md" ?disabled=${this.disabled || this.busy} @click=${this.send}>
                    Send ${iconLoader(Send16, { slot: 'icon' })}
                </cds-button>
            </div>
        `;
    }

    private fire(name: string, detail?: unknown) {
        this.dispatchEvent(new CustomEvent(name, { detail }));
    }

    // upload fires the event to upload the files, and attach them, unless
    // there are none, or the box is disabled
    private upload(files: File[]) {
        if (files.length > 0 && !this.disabled) {
            this.fire(ChatInput.eventUpload, files);
        }
    }

    // handlePaste uploads the files which are pasted, such as a screenshot,
    // rather than pasting their names
    private handlePaste(event: ClipboardEvent) {
        const files = Array.from(event.clipboardData?.files ?? []);
        if (files.length > 0) {
            event.preventDefault();
            this.upload(files);
        }
    }

    // handleDrag accepts files which are dragged over the box
    private readonly handleDrag = (event: DragEvent) => {
        if (this.disabled || !hasFiles(event)) {
            return;
        }
        event.preventDefault();
        if (event.dataTransfer) {
            event.dataTransfer.dropEffect = 'copy';
        }
        this.dragging = true;
    };

    private readonly handleDragLeave = (event: DragEvent) => {
        // Leaving the box for an element in it isn't leaving it
        if (!(event.relatedTarget instanceof Node && this.shadowRoot?.contains(event.relatedTarget)) && event.relatedTarget !== this) {
            this.dragging = false;
        }
    };

    private readonly handleDrop = (event: DragEvent) => {
        if (!hasFiles(event)) {
            return;
        }
        event.preventDefault();
        this.dragging = false;
        this.upload(Array.from(event.dataTransfer?.files ?? []));
    };

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

    // send fires the event with the message, unless it's empty, and has no
    // attachments, and empties the box
    private send() {
        const text = this.textarea.value.trim();
        if (this.disabled || this.busy || (!text && this.attachments.length === 0)) {
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
