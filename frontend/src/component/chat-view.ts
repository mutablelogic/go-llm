// The conversation of a session, as chat bubbles: the person's questions on
// the right, and the assistant's replies on the left, as Markdown, with its
// thinking, and the tools it called, and their results, as thinking, which
// is collapsed, under the names of the person and the assistant, where who's
// speaking changes. The reply which is streamed is live: its text, and its
// thinking, as it's generated, with what tools log, and how they're getting
// on, dimmed between them. A bubble's details action fires an event with its
// message. Beneath it is the box which sends a question, and says someone is
// typing. It fires events to send a question, and say someone is typing.

import { LitElement, css, html, nothing, type PropertyValues } from 'lit';
import { customElement, property, query } from 'lit/decorators.js';
import { canonicalTool, isUser, type ContentBlock, type Message, type Session } from '../schema/session';
import { detailValue, type Details } from '../schema/memory';
import { markdown, markdownStyles } from './markdown';
import type { ChatKind } from './chat-bubble';
import './chat-bubble';
import { ChatInput } from './chat-input';
import './chat-input';

// Carbon components
import '@carbon/web-components/es/components/skeleton-text/index.js';

// Piece is a part of a reply as it's streamed: its text, its thinking, or a
// line a tool logged
export interface Piece {
    what: 'text' | 'thinking' | 'log';
    text: string;
}

// Bubble is a bubble of a message, with its kind, its label when it's
// thinking, its text, whether its text is Markdown, and whether it's live,
// as it's streamed, or a line which a tool logged, which isn't a bubble
interface Bubble {
    kind: ChatKind | 'log';
    label?: string;
    text: string;
    markdown?: boolean;
    live?: boolean;
    message?: Message;
}

@customElement('llm-chat-view')
export class ChatView extends LitElement {
    // The names of the events fired to send a question, whose detail is its
    // text, to say someone is typing, to delete the session, and to show a
    // message's details, whose detail is the message
    static readonly eventSend = ChatInput.eventSend;
    static readonly eventTyping = ChatInput.eventTyping;
    static readonly eventInfo = 'llm-chat-info';

    // The session, which is undefined when none is open
    @property({ attribute: false })
    session?: Session;

    // Its messages, which are undefined while they're loading
    @property({ attribute: false })
    messages?: Message[];

    // The lines tools logged, as replies were streamed, by the identifier of
    // a tool's result, which they're shown before, or the position of a
    // message, which they're shown after
    @property({ attribute: false })
    logs: (result: string | undefined, position: number) => string[] | undefined = () => undefined;

    // The message whose details are shown, whose bubbles are selected
    @property({ attribute: false })
    selected?: Message;

    // The memory's details, for the names of the person and the assistant
    @property({ attribute: false })
    details: Details = {};

    // The question which is waiting for its reply, and the reply, as it's
    // streamed, and how a tool is getting on, which replaces the one before
    @property({ attribute: false })
    question?: string;

    @property({ attribute: false })
    streamed: Piece[] = [];

    @property({ attribute: false })
    progress?: string;

    // Whether the server can't be reached
    @property({ type: Boolean, reflect: true })
    unavailable = false;

    @query('.conversation')
    private conversation?: HTMLElement;

    @query('llm-chat-input')
    private input?: ChatInput;

    // Whether the conversation was scrolled to its end, before it changed,
    // so it follows the reply, unless it was scrolled up to read
    private following = true;

    static styles = [markdownStyles, css`
        :host {
            display: flex;
            flex-direction: column;
            min-height: 0;
            min-width: 0;
        }

        .conversation {
            flex: 1;
            overflow-y: auto;
        }

        .messages {
            display: flex;
            flex-direction: column;
            gap: 0.75rem;
            padding: 1.5rem 2rem;
        }

        /* Who's speaking, where it changes, above their bubbles, on their
           side */
        .name {
            margin: 0.5rem 0 -0.5rem;
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
        }

        .name.user {
            align-self: flex-end;
        }

        /* A question keeps its line breaks, and a reply is Markdown */
        span.text {
            white-space: pre-wrap;
        }

        /* What tools log, and how they're getting on, between the bubbles */
        .log {
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
            padding-left: 1rem;
        }

        .empty {
            margin: 0;
            padding: 1.5rem 2rem;
            color: var(--cds-text-secondary);
        }

        .none {
            margin: auto;
            color: var(--cds-text-secondary);
            text-align: center;
        }

        /* An event's session has a note where the box which sends a
           question would be */
        .event-note {
            padding: 1rem 2rem;
            border-top: 1px solid var(--cds-border-subtle-01);
            color: var(--cds-text-secondary);
            font-size: 0.875rem;
        }

        cds-skeleton-text {
            padding: 1.5rem 2rem;
        }

        :host([unavailable]) .conversation {
            opacity: 0.5;
        }
    `];

    // focus focuses the box which sends a question
    focus() {
        this.input?.focus();
    }

    // restore puts the question back in the box, such as when it couldn't be
    // sent, unless another has been started
    restore(text: string) {
        this.input?.restore(text);
    }

    willUpdate(_changed: PropertyValues<this>) {
        const el = this.conversation;
        this.following = !el || el.scrollTop + el.clientHeight >= el.scrollHeight - 48;
    }

    updated(changed: PropertyValues<this>) {
        // A session which is opened is shown from its end, once its bubbles
        // are laid out
        if (changed.has('session') && changed.get('session')?.id !== this.session?.id) {
            this.following = true;
        }
        if (this.following) {
            requestAnimationFrame(() => {
                if (this.conversation) {
                    this.conversation.scrollTop = this.conversation.scrollHeight;
                }
            });
        }
    }

    render() {
        if (!this.session) {
            return html`<div class="none">Choose a chat, or start a new one</div>`;
        }
        // An event's session, such as a heartbeat's, has what it did each
        // time it ran, and isn't chatted in
        if (this.session.event) {
            return html`
                <div class="conversation">${this.renderConversation()}</div>
                <div class="event-note">This is an event's session: what a reminder did each time it fired. It isn't chatted in.</div>
            `;
        }
        return html`
            <div class="conversation">${this.renderConversation()}</div>
            <llm-chat-input ?disabled=${this.messages === undefined} ?busy=${this.question !== undefined}
                @llm-chat-send=${(e: CustomEvent<string>) => this.dispatchEvent(new CustomEvent(ChatView.eventSend, { detail: e.detail }))}
                @llm-chat-typing=${() => this.dispatchEvent(new CustomEvent(ChatView.eventTyping))}></llm-chat-input>
        `;
    }

    // renderConversation returns the bubbles of the messages, and of the
    // question which is waiting for its reply, and the reply, as it's
    // streamed, with the name of who's speaking where it changes
    private renderConversation() {
        if (!this.messages) {
            return html`<cds-skeleton-text paragraph line-count="6"></cds-skeleton-text>`;
        }
        const bubbles = this.messages.flatMap((message, position) => this.messageBubbles(message, position));
        if (this.question !== undefined) {
            bubbles.push({ kind: 'user', text: this.question }, ...this.streamedBubbles());
        }
        if (bubbles.length === 0) {
            return html`<p class="empty">No messages yet. Ask a question to start the conversation.</p>`;
        }
        let speaker: 'user' | 'assistant' | undefined;
        return html`<div class="messages">${bubbles.map((bubble) => {
            const from = bubble.kind === 'user' ? 'user' : 'assistant';
            const name = from !== speaker ? html`<div class="name ${from}">${from === 'user' ? this.userName() : this.assistantName()}</div>` : nothing;
            speaker = from;
            return html`${name}${this.renderBubble(bubble)}`;
        })}</div>`;
    }

    private renderBubble(bubble: Bubble) {
        if (bubble.kind === 'log') {
            return html`<div class="log">${bubble.text}</div>`;
        }
        const message = bubble.message;
        return html`
            <llm-chat-bubble kind=${bubble.kind} label=${bubble.label ?? 'Thinking'} ?live=${bubble.live === true}
                ?selected=${message !== undefined && message === this.selected}
                @llm-chat-bubble-info=${() => message && this.dispatchEvent(new CustomEvent(ChatView.eventInfo, { detail: message }))}
                .copyText=${bubble.text}>${bubble.markdown
                    ? html`<div class="markdown">${markdown(bubble.text)}</div>`
                    : html`<span class="text">${bubble.text}</span>`}</llm-chat-bubble>
        `;
    }

    // messageBubbles returns the bubbles of a message, with the lines tools
    // logged, before the results of the tools they're kept by, and after the
    // message they're kept by the position of
    private messageBubbles(message: Message, position: number): Bubble[] {
        const bubbles: Bubble[] = [];
        for (const block of message.content) {
            const id = block.tool_result?.id;
            for (const line of (id && this.logs(id, position)) || []) {
                bubbles.push({ kind: 'log', text: line });
            }
        }
        bubbles.push(...bubblesOf(message));
        for (const line of this.logs(undefined, position) ?? []) {
            bubbles.push({ kind: 'log', text: line });
        }
        return bubbles;
    }

    // streamedBubbles returns the bubbles of the reply, as it's streamed: its
    // text, and its thinking, joined while they follow each other, the lines
    // tools logged, and how a tool is getting on, of which the last bubble
    // is live, or an ellipsis, until there's something
    private streamedBubbles(): Bubble[] {
        const bubbles: Bubble[] = [];
        for (const piece of this.streamed) {
            const last = bubbles[bubbles.length - 1];
            const kind = piece.what === 'text' ? 'assistant' : piece.what === 'thinking' ? 'thinking' : 'log';
            if (last && last.kind === kind && kind !== 'log') {
                last.text += piece.text;
            } else {
                bubbles.push({ kind, text: piece.text, markdown: kind === 'assistant' });
            }
        }
        const live = [...bubbles].reverse().find((bubble) => bubble.kind !== 'log');
        if (live) {
            live.live = true;
        }
        if (this.progress) {
            bubbles.push({ kind: 'log', text: this.progress });
        }
        if (!live) {
            bubbles.push({ kind: 'assistant', text: '…', live: true });
        }
        return bubbles;
    }

    // userName returns the person's name, when it's known, or You
    private userName(): string {
        return detailValue(this.details.name) ?? 'You';
    }

    // assistantName returns the assistant's name, when it's known, or the
    // provider of the session's model, such as ollama, or Assistant
    private assistantName(): string {
        const provider = this.session?.model?.split(':')[0];
        return detailValue(this.details.assistant) ?? (provider || 'Assistant');
    }
}

// bubblesOf returns the bubbles of a message: the person's text, the
// assistant's text, as Markdown, its thinking, and the tools it called, and
// their results, as thinking, by the tools' canonical names, such as
// weatherapi:current, and why a reply stopped, when it didn't finish. A
// system message isn't shown.
function bubblesOf(message: Message): Bubble[] {
    if (message.role === 'system') {
        return [];
    }
    const kind: ChatKind = isUser(message) ? 'user' : 'assistant';
    const bubbles = message.content.flatMap((block) => blockBubbles(kind, block)).map((bubble) => ({ ...bubble, message }));
    if (message.result && !['stop', 'tool_call'].includes(message.result)) {
        bubbles.push({ kind: 'thinking', label: `The reply stopped: ${message.result}`, text: '', message });
    }
    return bubbles;
}

function blockBubbles(kind: ChatKind, block: ContentBlock): Bubble[] {
    const bubbles: Bubble[] = [];
    if (block.thinking?.trim()) {
        bubbles.push({ kind: 'thinking', text: block.thinking.trim() });
    }
    if (block.text?.trim()) {
        bubbles.push({ kind, text: block.text, markdown: kind === 'assistant' });
    }
    if (block.attachment) {
        bubbles.push({ kind, text: `Attachment: ${block.attachment.type ?? block.attachment.url ?? ''}` });
    }
    if (block.tool_call) {
        bubbles.push({ kind: 'thinking', label: `Called ${canonicalTool(block.tool_call.name)}`, text: json(block.tool_call.input) });
    }
    if (block.tool_result) {
        const name = canonicalTool(block.tool_result.name ?? 'the tool');
        bubbles.push({ kind: 'thinking', label: block.tool_result.is_error ? `${name} failed` : `Result of ${name}`, text: json(block.tool_result.content) });
    }
    return bubbles;
}

// json returns a value as indented JSON, or text when it's a string
function json(value: unknown): string {
    if (value === undefined || value === null) {
        return '';
    }
    return typeof value === 'string' ? value : JSON.stringify(value, null, 2);
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-chat-view': ChatView;
    }
}
