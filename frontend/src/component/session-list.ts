// The list of sessions, most recently changed first, each with its model,
// when it was last changed, the tokens of its conversation, and whether
// someone is typing in it, or its model is replying. Clicking a session
// fires an event, so it's opened, and the session which is open is
// selected.

import { LitElement, css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { sessionTokens, type Session } from '../schema/session';
import { formatAgo, formatTime } from '../format';
import { MinuteController } from './minute';

// Carbon components
import '@carbon/web-components/es/components/skeleton-text/index.js';
import '@carbon/web-components/es/components/tag/tag.js';

@customElement('llm-session-list')
export class SessionList extends LitElement {
    // The name of the event fired when a session is clicked, whose detail
    // is the session
    static readonly eventSelect = 'llm-session-select';

    // The sessions, which are undefined while they're loading
    @property({ attribute: false })
    sessions?: Session[];

    // The identifier of the session which is open
    @property({ attribute: false })
    selected?: string;

    // Whether the server can't be reached
    @property({ type: Boolean, reflect: true })
    unavailable = false;

    static styles = css`
        :host {
            display: flex;
            flex-direction: column;
            min-height: 0;
            background: var(--cds-layer-01);
            border-right: 1px solid var(--cds-border-subtle-01);
        }

        ul {
            flex: 1;
            margin: 0;
            padding: 0;
            overflow-y: auto;
            list-style: none;
        }

        button.session {
            all: unset;
            box-sizing: border-box;
            display: block;
            width: 100%;
            padding: 0.75rem 1rem;
            border-bottom: 1px solid var(--cds-border-subtle-01);
            cursor: pointer;
        }

        button.session:hover {
            background: var(--cds-layer-hover-01);
        }

        button.session:focus-visible {
            outline: 2px solid var(--cds-focus);
            outline-offset: -2px;
        }

        button.session[aria-current='true'] {
            background: var(--cds-layer-selected-01);
            box-shadow: inset 3px 0 0 var(--cds-interactive);
        }

        .model {
            display: block;
            font-size: 0.875rem;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        .secondary {
            display: block;
            margin-top: 0.125rem;
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
        }

        .status {
            color: var(--cds-support-info);
        }

        .empty {
            padding: 1rem;
            color: var(--cds-text-secondary);
            font-size: 0.875rem;
        }

        :host([unavailable]) ul {
            opacity: 0.5;
        }
    `;

    // The list is rendered each minute, as when sessions changed is shown
    // relative to now
    private readonly minute = new MinuteController(this);

    render() {
        return this.renderSessions();
    }

    private renderSessions() {
        if (!this.sessions) {
            return html`<div class="empty"><cds-skeleton-text paragraph line-count="4"></cds-skeleton-text></div>`;
        } else if (this.sessions.length === 0) {
            return html`<div class="empty">No chats yet</div>`;
        }
        return html`<ul>${this.sessions.map((session) => this.renderSession(session))}</ul>`;
    }

    private renderSession(session: Session) {
        const tokens = sessionTokens(session);
        const status = session.status === 'replying' ? 'replying…' : session.status === 'typing' ? 'typing…' : '';
        const when = session.modified ?? session.created;
        return html`
            <li>
                <button class="session" type="button" aria-current=${session.id === this.selected ? 'true' : 'false'}
                    @click=${() => this.dispatchEvent(new CustomEvent(SessionList.eventSelect, { detail: session }))}>
                    <span class="model">${session.model ?? 'No model'}${session.event ? html` <cds-tag size="sm" type="purple">event</cds-tag>` : nothing}</span>
                    <span class="secondary" title=${formatTime(when)}>
                        ${status ? html`<span class="status">${status}</span>` : formatAgo(when)}
                        ${tokens ? html` · ${tokens.toLocaleString()} tokens` : nothing}
                    </span>
                </button>
            </li>
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-session-list': SessionList;
    }
}
