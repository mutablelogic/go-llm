// The conversation of a session, as chat bubbles: the person's questions on
// the right, and the assistant's replies on the left, as Markdown, with its
// thinking, and the tools it called, and their results, as thinking, which
// is collapsed, under the names of the person and the assistant, where who's
// speaking changes. The reply which is streamed is live: its text, and its
// thinking, as it's generated, with what tools log, and how they're getting
// on, dimmed between them. A bubble's details action fires an event with its
// message. Beneath it is the box which sends a question, and says someone is
// typing, with the resources which are attached to it. It fires events to
// send a question, say someone is typing, choose resources to attach,
// remove an attachment, and upload files to attach.

import './welcome';
import type { Provider } from '../schema/provider';
import type { Model } from '../schema/model';
import { LitElement, css, html, nothing, type PropertyValues } from 'lit';
import { customElement, property, query } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Document16 from '@carbon/icons/es/document/16.js';
import DocumentPdf16 from '@carbon/icons/es/document--pdf/16.js';
import Image16 from '@carbon/icons/es/image/16.js';
import Music16 from '@carbon/icons/es/music/16.js';
import Video16 from '@carbon/icons/es/video/16.js';
import PlayFilledAlt16 from '@carbon/icons/es/play--filled--alt/16.js';
import { MediaPlayer, playable, type Media } from './media-player';
import { resourceName, type Resource, resourceHash } from '../schema/resource';
import { canonicalTool, isUser, type ContentBlock, type Marker, type Message, type Session } from '../schema/session';
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
    kind: ChatKind | 'log' | 'marker' | 'time';
    marker?: Marker;
    label?: string;
    text: string;
    attachment?: Attachment;
    media?: Attachment;
    markdown?: boolean;
    live?: boolean;
    message?: Message;
}

// Attachment is a message's attachment, as it's shown: its name, when it's
// known, or else its type, or URL, and its type, which chooses its icon
interface Attachment {
    name: string;
    type?: string;
    url?: string;
}

// AttachmentName returns the name of the resource an attachment refers to,
// by its URL, such as resource:///<hash>, when it's known
export type AttachmentName = (url: string) => string | undefined;

// ResourceType returns the type of the resource with the URI,
// resource:///<hash>, when it's known
export type ResourceType = (uri: string) => string | undefined;

// ContentURL returns the URL of the content of the resource with the URI,
// resource:///<hash>, which the browser plays, shows, or downloads, or
// undefined, when it isn't known
export type ContentURL = (uri: string) => string | undefined;


@customElement('llm-chat-view')
export class ChatView extends LitElement {
    // The names of the events fired to send a question, whose detail is its
    // text, to say someone is typing, to delete the session, and to show a
    // message's details, whose detail is the message
    static readonly eventSend = ChatInput.eventSend;
    static readonly eventTyping = ChatInput.eventTyping;
    static readonly eventInfo = 'llm-chat-info';

    // The name of the event fired as the conversation is scrolled up to its
    // first message, which is loaded, when there are earlier ones
    static readonly eventEarlier = 'llm-chat-earlier';
    static readonly eventAttach = ChatInput.eventAttach;
    static readonly eventDetach = ChatInput.eventDetach;
    static readonly eventUpload = ChatInput.eventUpload;

    // The session, which is undefined when none is open
    @property({ attribute: false })
    session?: Session;

    // Its messages, which are undefined while they're loading, from the
    // position of the first, of the session's, as earlier ones are loaded as
    // it's scrolled up to them, and whether they're loading
    @property({ attribute: false })
    messages?: Message[];

    @property({ attribute: false })
    offset = 0;

    @property({ attribute: false })
    earlierLoading = false;

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

    // The resources attached to the question which is waiting for its reply
    @property({ attribute: false })
    questionAttachments: Resource[] = [];

    // The names of the resources which messages' attachments refer to
    @property({ attribute: false })
    attachmentName: AttachmentName = () => undefined;

    // The URLs of the content of resources, which are played, or shown, by
    // the browser
    @property({ attribute: false })
    contentURL: ContentURL = () => undefined;

    // The types of resources, so a link to one in a reply, which is audio,
    // or video, is played in the player, rather than in a new tab
    @property({ attribute: false })
    resourceType: ResourceType = () => undefined;

    @property({ attribute: false })
    streamed: Piece[] = [];

    @property({ attribute: false })
    progress?: string;

    // The resources which are attached to the next question
    @property({ attribute: false })
    attachments: Resource[] = [];

    // How many files are being uploaded, to attach to it
    @property({ attribute: false })
    uploading = 0;

    // Whether the server can't be reached
    @property({ type: Boolean, reflect: true })
    unavailable = false;

    // The providers, and models, which the welcome shows, in place of a
    // chat, when there are none yet, or undefined, when there are chats
    @property({ attribute: false })
    welcome?: { providers?: Provider[]; models?: Model[] };

    @query('.conversation')
    private conversation?: HTMLElement;

    @query('llm-chat-input')
    private input?: ChatInput;

    // Whether the conversation was scrolled to its end, before it changed,
    // so it follows the reply, unless it was scrolled up to read
    private following = true;

    // The distance of what's shown from the end of the conversation, as
    // earlier messages are added, so it isn't moved by them
    private anchor?: number;

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

        /* An attachment is its icon, by its type, and its name */
        .attachment {
            display: inline-flex;
            align-items: center;
            gap: 0.375rem;
            overflow-wrap: anywhere;
        }

        .attachment a {
            color: inherit;
        }

        /* A resource a tool returned, such as speech, with its player, or
           an image */
        .media {
            display: flex;
            flex-direction: column;
            gap: 0.5rem;
        }

        /* A resource which is played, in the player, by its button */
        button.play {
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
            padding: 0.25rem 0.75rem 0.25rem 0.5rem;
            border: 1px solid var(--cds-border-strong-01);
            border-radius: 1rem;
            background: var(--cds-layer-01);
            color: var(--cds-text-primary);
            font: inherit;
            cursor: pointer;
        }

        button.play:hover {
            background: var(--cds-layer-hover-01);
        }

        button.play:focus-visible {
            outline: 2px solid var(--cds-focus, #0f62fe);
        }

        button.play svg {
            fill: currentColor;
        }

        .media img {
            display: block;
            max-width: min(24rem, 100%);
            max-height: 20rem;
        }

        .attachment svg {
            flex: none;
            fill: currentColor;
        }

        /* A question keeps its line breaks, and a reply is Markdown */
        span.text {
            white-space: pre-wrap;
        }

        /* What tools log, and how they're getting on, between the bubbles */
        .earlier {
            margin: 0.5rem 0;
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
            text-align: center;
        }

        .marker {
            margin: 1rem 0;
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
        }

        .marker .rule {
            display: flex;
            align-items: center;
            gap: 0.75rem;
        }

        .marker .rule::before,
        .marker .rule::after {
            content: '';
            flex: 1;
            border-top: 1px solid var(--cds-border-subtle);
        }

        .marker .rule span {
            flex: 0 1 auto;
            text-align: center;
        }

        .marker .rule::before,
        .marker .rule::after {
            min-width: 2rem;
        }

        .marker .note {
            margin-top: 0.25rem;
            text-align: center;
            font-size: 0.6875rem;
        }

        .marker details {
            margin: 0.5rem 1rem 0;
        }

        .marker summary {
            cursor: pointer;
            text-align: center;
        }

        .marker .markdown {
            margin-top: 0.5rem;
            color: var(--cds-text-primary);
            font-size: 0.875rem;
        }

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

    willUpdate(changed: PropertyValues<this>) {
        const el = this.conversation;
        this.following = !el || el.scrollTop + el.clientHeight >= el.scrollHeight - 48;

        // Earlier messages which are added keep what's shown where it is,
        // by its distance from the end
        const before = changed.get('offset') as number | undefined;
        this.anchor = el && before !== undefined && this.offset < before && !changed.has('session') ? el.scrollHeight - el.scrollTop : undefined;
    }

    // handleScroll asks for earlier messages, as the conversation is scrolled
    // near its top, when there are any, and they aren't loading
    private handleScroll = () => {
        const el = this.conversation;
        if (el && el.scrollTop < 200 && this.offset > 0 && !this.earlierLoading && this.messages) {
            this.dispatchEvent(new CustomEvent(ChatView.eventEarlier));
        }
    };

    updated(changed: PropertyValues<this>) {
        // A session which is opened is shown from its end, once its bubbles
        // are laid out
        if (changed.has('session') && changed.get('session')?.id !== this.session?.id) {
            this.following = true;
        }
        // Earlier messages are asked for, when those which are loaded don't
        // fill the conversation, so it can't be scrolled up to them, once
        // they change, rather than on every update, so a page which couldn't
        // be loaded isn't asked for again, until it's scrolled to
        if (changed.has('messages') || changed.has('offset')) {
            requestAnimationFrame(() => {
                const el = this.conversation;
                if (el && el.scrollHeight <= el.clientHeight + 200) {
                    this.handleScroll();
                }
            });
        }
        if (this.anchor !== undefined && this.conversation) {
            this.conversation.scrollTop = this.conversation.scrollHeight - this.anchor;
            this.anchor = undefined;
        } else if (this.following) {
            requestAnimationFrame(() => {
                if (this.conversation) {
                    this.conversation.scrollTop = this.conversation.scrollHeight;
                }
            });
        }
    }

    render() {
        if (!this.session) {
            return this.welcome
                ? html`<llm-welcome .providers=${this.welcome.providers} .models=${this.welcome.models}></llm-welcome>`
                : html`<div class="none">Choose a chat, or start a new one</div>`;
        }
        // An event's session, such as a heartbeat's, has what it did each
        // time it ran, and isn't chatted in
        if (this.session.event) {
            return html`
                <div class="conversation" @scroll=${this.handleScroll}>${this.renderConversation()}</div>
                <div class="event-note">This is an event's session: what a reminder did each time it fired. It isn't chatted in.</div>
            `;
        }
        return html`
            <div class="conversation" @scroll=${this.handleScroll}>${this.renderConversation()}</div>
            <llm-chat-input ?disabled=${this.messages === undefined} ?busy=${this.question !== undefined} .attachments=${this.attachments} .uploading=${this.uploading}
                @llm-chat-send=${(e: CustomEvent<string>) => this.dispatchEvent(new CustomEvent(ChatView.eventSend, { detail: e.detail }))}
                @llm-chat-typing=${() => this.dispatchEvent(new CustomEvent(ChatView.eventTyping))}
                @llm-chat-attach=${() => this.dispatchEvent(new CustomEvent(ChatView.eventAttach))}
                @llm-chat-detach=${(e: CustomEvent<Resource>) => this.dispatchEvent(new CustomEvent(ChatView.eventDetach, { detail: e.detail }))}
                @llm-chat-upload=${(e: CustomEvent<File[]>) => this.dispatchEvent(new CustomEvent(ChatView.eventUpload, { detail: e.detail }))}></llm-chat-input>
        `;
    }

    // renderConversation returns the bubbles of the messages, and of the
    // question which is waiting for its reply, and the reply, as it's
    // streamed, with the name of who's speaking where it changes
    private renderConversation() {
        if (!this.messages) {
            return html`<cds-skeleton-text paragraph line-count="6"></cds-skeleton-text>`;
        }
        // A divider says when the conversation was, before the first message,
        // and each which starts another hour, in the person's time zone
        const zone = timeZone(detailValue(this.details.timezone));
        let hour: string | undefined;
        const bubbles = this.messages.flatMap((message, position) => {
            // A message which shows nothing, such as a system message, has
            // no divider, and doesn't start an hour
            const shown = this.messageBubbles(message, this.offset + position);
            if (shown.length === 0) {
                return [];
            }
            const when = message.time ? new Date(message.time) : undefined;
            const key = when && !isNaN(when.getTime()) ? hourKey(when, zone) : undefined;
            const divider: Bubble[] = key && key !== hour ? [{ kind: 'time', text: timeLabel(when!, zone) }] : [];
            hour = key ?? hour;
            return [...divider, ...shown];
        });
        if (this.question !== undefined) {
            bubbles.push(...this.questionAttachments.map((resource): Bubble => ({
                kind: 'user',
                text: resourceName(resource),
                attachment: { name: resourceName(resource), type: resource.type },
            })));
            if (this.question) {
                bubbles.push({ kind: 'user', text: this.question });
            }
            bubbles.push(...this.streamedBubbles());
        }
        if (bubbles.length === 0) {
            return html`<p class="empty">No messages yet. Ask a question to start the conversation.</p>`;
        }
        let speaker: 'user' | 'assistant' | undefined;
        const earlier = this.offset > 0
            ? html`<div class="earlier">${this.earlierLoading ? 'Loading earlier messages…' : 'Scroll up for earlier messages'}</div>`
            : nothing;
        return html`${earlier}<div class="messages">${bubbles.map((bubble) => {
            if (bubble.kind === 'marker') {
                // Whoever speaks next is named again, after the divider
                speaker = undefined;
                return this.renderMarker(bubble.marker!);
            } else if (bubble.kind === 'time') {
                speaker = undefined;
                return html`<div class="marker time"><div class="rule"><span>${bubble.text}</span></div></div>`;
            }
            const from = bubble.kind === 'user' ? 'user' : 'assistant';
            const name = from !== speaker ? html`<div class="name ${from}">${from === 'user' ? this.userName() : this.assistantName()}</div>` : nothing;
            speaker = from;
            return html`${name}${this.renderBubble(bubble)}`;
        })}</div>`;
    }

    // renderMarker returns the divider where the conversation was compacted,
    // which says what became of the messages before it, with their summary,
    // which can be shown, once it's written
    private renderMarker(marker: Marker) {
        const label = marker.status === 'pending'
            ? 'Summarising the messages above'
            : marker.status === 'failed'
            ? "The messages above couldn't be summarised"
            : marker.reason === 'full'
            ? 'The chat was full, so the messages above were summarised'
            : 'The messages above were summarised';
        return html`
            <div class="marker">
                <div class="rule"><span>${label}</span></div>
                <div class="note">${marker.status === 'failed' ? `So they're still sent${marker.error ? `: ${marker.error}` : ''}` : "They're kept, but no longer sent"}</div>
                ${marker.summary ? html`
                    <details>
                        <summary>What the assistant remembers of them</summary>
                        <div class="markdown">${markdown(marker.summary)}</div>
                    </details>
                ` : nothing}
            </div>
        `;
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
                .copyText=${bubble.text}>${bubble.media
                    ? this.renderMedia(bubble.media)
                    : bubble.attachment
                    ? this.renderAttachment(bubble.attachment)
                    : bubble.markdown
                    ? html`<div class="markdown" @click=${this.handleLink}>${markdown(bubble.text, this.contentURL)}</div>`
                    : html`<span class="text">${bubble.text}</span>`}</llm-chat-bubble>
        `;
    }

    // renderAttachment returns an attachment: its icon, and its name, which
    // links to its content, when it's known
    private renderAttachment(attachment: Attachment) {
        const url = attachment.url ? this.contentURL(attachment.url) : undefined;
        const name = url
            ? html`<a href=${url} target="_blank" rel="noopener noreferrer"
                @click=${(e: Event) => this.playLink(e, { url, name: attachment.name, type: attachment.type })}>${attachment.name}</a>`
            : html`<span>${attachment.name}</span>`;
        return html`<span class="attachment">${iconLoader(attachmentIcon(attachment.type))}${name}</span>`;
    }

    // renderMedia returns a resource a tool returned, such as speech: a
    // player of audio, or video, an image, or otherwise its name, which links
    // to its content
    private renderMedia(media: Attachment) {
        const url = media.url ? this.contentURL(media.url) : undefined;
        const mediaType = (media.type ?? '').split(';')[0].trim().toLowerCase();
        if (!url) {
            return this.renderAttachment(media);
        } else if (playable(mediaType)) {
            return html`
                <button class="play" @click=${() => this.play({ url, name: media.name, type: media.type })}>
                    ${iconLoader(PlayFilledAlt16)}<span>${media.name}</span>
                </button>`;
        } else if (mediaType.startsWith('image/') && mediaType !== 'image/svg+xml') {
            return html`<div class="media">${this.renderAttachment(media)}<a href=${url} target="_blank" rel="noopener noreferrer"><img src=${url} alt=${media.name}></a></div>`;
        }
        return this.renderAttachment(media);
    }

    // play plays the media in the player
    private play(media: Media) {
        this.dispatchEvent(new CustomEvent(MediaPlayer.eventPlay, { detail: media, bubbles: true, composed: true }));
    }

    // playLink plays the media a link is to in the player, rather than in a
    // new tab, when it's audio, or video
    private playLink(event: Event, media: Media) {
        if (playable(media.type)) {
            event.preventDefault();
            this.play(media);
        }
    }

    // handleLink plays a link in a reply to a resource which is audio, or
    // video, in the player, rather than in a new tab
    private readonly handleLink = (event: Event) => {
        const link = (event.target as Element | null)?.closest?.('a');
        const match = link?.getAttribute('href')?.match(/\/resource\/([0-9a-f]{64})\/content/);
        if (!link || !match) {
            return;
        }
        const uri = 'resource:///' + match[1];
        this.playLink(event, { url: link.getAttribute('href')!, name: link.textContent?.trim() || this.attachmentName(uri) || 'Media', type: this.resourceType(uri) });
    };


    // messageBubbles returns the bubbles of a message, with the lines tools
    // logged, before the results of the tools they're kept by, and after the
    // message they're kept by the position of
    private messageBubbles(message: Message, position: number): Bubble[] {
        const bubbles: Bubble[] = [];

        // An event's outcome, such as a reminder, which was delivered to the
        // chat, says which event it's of
        if (message.event) {
            bubbles.push({ kind: 'log', text: `⏰ ${message.event.title || 'A heartbeat'}` });
        }
        for (const block of message.content) {
            const id = block.tool_result?.id;
            for (const line of (id && this.logs(id, position)) || []) {
                bubbles.push({ kind: 'log', text: line });
            }
        }
        bubbles.push(...bubblesOf(message, this.attachmentName));
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
function bubblesOf(message: Message, attachmentName: AttachmentName): Bubble[] {
    if (message.role === 'system') {
        return [];
    } else if (message.marker) {
        return [{ kind: 'marker', text: '', marker: message.marker, message }];
    }
    const kind: ChatKind = isUser(message) ? 'user' : 'assistant';
    const bubbles = message.content.flatMap((block) => blockBubbles(kind, block, attachmentName)).map((bubble) => ({ ...bubble, message }));
    if (message.result && !['stop', 'tool_call'].includes(message.result)) {
        bubbles.push({ kind: 'thinking', label: `The reply stopped: ${message.result}`, text: '', message });
    }
    return bubbles;
}

function blockBubbles(kind: ChatKind, block: ContentBlock, attachmentName: AttachmentName): Bubble[] {
    const bubbles: Bubble[] = [];
    if (block.thinking?.trim()) {
        bubbles.push({ kind: 'thinking', text: block.thinking.trim() });
    }
    if (block.text?.trim()) {
        bubbles.push({ kind, text: block.text, markdown: kind === 'assistant' });
    }
    if (block.attachment) {
        const { url, type } = block.attachment;
        const name = (url && attachmentName(url)) || type || url || 'Attachment';
        // What the assistant made, such as an image, is shown, or played,
        // and the person's attachments are their names
        if (kind === 'assistant') {
            bubbles.push({ kind, text: name, media: { name, type, url } });
        } else {
            bubbles.push({ kind, text: name, attachment: { name, type, url } });
        }
    }
    if (block.tool_call) {
        bubbles.push({ kind: 'thinking', label: `Called ${canonicalTool(block.tool_call.name)}`, text: json(block.tool_call.input) });
    }
    if (block.tool_result) {
        const name = canonicalTool(block.tool_result.name ?? 'the tool');
        bubbles.push({ kind: 'thinking', label: block.tool_result.is_error ? `${name} failed` : `Result of ${name}`, text: json(block.tool_result.content) });

        // The resources it returned, such as speech, after it, so they're
        // played, or shown, rather than in its result, which is collapsed
        for (const resource of resultResources(block.tool_result.content)) {
            const name = resource.title || resource.name || resource.type || 'Resource';
            bubbles.push({ kind: 'assistant', text: name, media: { name, type: resource.type, url: resource.uri } });
        }
    }
    return bubbles;
}

// resultResources returns the links to the resources a tool's result has,
// which is {"resources": [{"uri", "name", "type"}]}, or none
function resultResources(content: unknown): { uri: string; name?: string; title?: string; type?: string }[] {
    const resources = (content as { resources?: unknown } | null)?.resources;
    if (!Array.isArray(resources)) {
        return [];
    }
    return resources.filter((resource): resource is { uri: string; name?: string; title?: string; type?: string } =>
        typeof resource?.uri === 'string' && resourceHash(resource.uri) !== undefined);
}

// attachmentIcon returns the icon of an attachment of the type: an image,
// audio, video, a PDF, or else a document
function attachmentIcon(type?: string) {
    const mediaType = (type ?? '').split(';')[0].trim().toLowerCase();
    if (mediaType.startsWith('image/')) {
        return Image16;
    } else if (mediaType.startsWith('audio/')) {
        return Music16;
    } else if (mediaType.startsWith('video/')) {
        return Video16;
    } else if (mediaType === 'application/pdf') {
        return DocumentPdf16;
    }
    return Document16;
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

// timeZone returns the time zone, such as Europe/Berlin, when the browser
// knows it, or undefined, for the browser's own
function timeZone(zone?: string): string | undefined {
    if (!zone) {
        return undefined;
    }
    try {
        new Intl.DateTimeFormat(undefined, { timeZone: zone });
        return zone;
    } catch {
        return undefined;
    }
}

// hourKey returns the hour of the time, in the time zone, with the zone's
// offset, such as 2026-11-09T14+01:00, so messages in the same hour have
// the same key, and an hour which happens twice, as the clocks go back,
// has two
function hourKey(when: Date, zone?: string): string {
    const parts = new Intl.DateTimeFormat('en-CA', { timeZone: zone, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', hourCycle: 'h23', timeZoneName: 'longOffset' }).formatToParts(when);
    const part = (type: string) => parts.find((p) => p.type === type)?.value ?? '';
    return `${part('year')}-${part('month')}-${part('day')}T${part('hour')}${part('timeZoneName')}`;
}

// localDate returns the date of the time, in the time zone, such as
// 2026-11-09
function localDate(when: Date, zone?: string): string {
    return new Intl.DateTimeFormat('en-CA', { timeZone: zone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(when);
}

// dayBefore returns the date before the date, such as 2026-11-08 for
// 2026-11-09, by the calendar, rather than a day of hours earlier, which
// isn't yesterday as the clocks change
function dayBefore(date: string): string {
    const [year, month, day] = date.split('-').map(Number);
    return new Date(Date.UTC(year, month - 1, day - 1)).toISOString().slice(0, 10);
}

// timeLabel returns what the divider before a message says of its hour, in
// the time zone: Today, or Yesterday, or its day, and date, with its year,
// when it isn't this year, and the hour, which is the time of the message,
// in the time zone, with its minutes as 00, such as "Yesterday, 14:00"
function timeLabel(when: Date, zone?: string): string {
    const day = localDate(when, zone);
    const today = localDate(new Date(), zone);
    const hour = new Intl.DateTimeFormat(undefined, { timeZone: zone, hour: '2-digit', minute: '2-digit' })
        .formatToParts(when)
        .map((part) => part.type === 'minute' ? '00' : part.value)
        .join('');
    let date: string;
    if (day === today) {
        date = 'Today';
    } else if (day === dayBefore(today)) {
        date = 'Yesterday';
    } else {
        const sameYear = day.slice(0, 4) === today.slice(0, 4);
        date = new Intl.DateTimeFormat(undefined, { timeZone: zone, weekday: 'long', day: 'numeric', month: 'long', year: sameYear ? undefined : 'numeric' }).format(when);
    }
    return `${date}, ${hour}`;
}
