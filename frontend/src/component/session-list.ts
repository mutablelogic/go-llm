// The list of sessions, most recently changed first, each with its title,
// or its model, when it has none, its model, beneath its title, when it has
// one, when it was last changed, the tokens of its conversation, and whether
// someone is typing in it, or its model is replying. Clicking a session
// fires an event, so it's opened, and the session which is open is
// selected, with its actions on its right: its events, editing it, and
// deleting it, apart from an event's session, which has none.

import { LitElement, css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { sessionTokens, type Session } from '../schema/session';
import { formatAgo, formatTime } from '../format';
import { MinuteController } from './minute';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import TrashCan16 from '@carbon/icons/es/trash-can/16.js';
import Edit16 from '@carbon/icons/es/edit/16.js';
import Notification16 from '@carbon/icons/es/notification/16.js';

// Carbon components
import '@carbon/web-components/es/components/skeleton-text/index.js';
import '@carbon/web-components/es/components/tag/tag.js';
import '@carbon/web-components/es/components/icon-button/index.js';

@customElement('llm-session-list')
export class SessionList extends LitElement {
    // The name of the event fired when a session is clicked, whose detail
    // is the session
    static readonly eventSelect = 'llm-session-select';

    // The names of the events fired by the actions of the session which is
    // open, whose detail is the session
    static readonly eventEvents = 'llm-session-events';
    static readonly eventEdit = 'llm-session-edit';
    static readonly eventDelete = 'llm-session-delete';

    // The sessions, which are undefined while they're loading
    @property({ attribute: false })
    sessions?: Session[];

    // The identifier of the session which is open
    @property({ attribute: false })
    selected?: string;

    // Whether the server can't be reached
    @property({ type: Boolean, reflect: true })
    unavailable = false;

    // Whether the Events action is red, as an event for the session which is
    // open was just created, which it is for a few seconds
    @property({ type: Boolean })
    alerted = false;

    // How long the Events action is red, and the timer which ends it
    static readonly alertDuration = 5000;
    private alertTimer?: ReturnType<typeof setTimeout>;

    // alert makes the Events action red, for a few seconds, as an event for
    // the session which is open was just created
    alert() {
        this.alerted = true;
        clearTimeout(this.alertTimer);
        this.alertTimer = setTimeout(() => {
            this.alerted = false;
        }, SessionList.alertDuration);
    }

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

        li {
            display: flex;
            align-items: center;
            border-bottom: 1px solid var(--cds-border-subtle-01);
        }

        li:hover {
            background: var(--cds-layer-hover-01);
        }

        li.selected {
            background: var(--cds-layer-selected-01);
            box-shadow: inset 3px 0 0 var(--cds-interactive);
        }

        button.session {
            all: unset;
            box-sizing: border-box;
            display: block;
            flex: 1;
            min-width: 0;
            padding: 0.75rem 1rem;
            cursor: pointer;
        }

        button.session:focus-visible {
            outline: 2px solid var(--cds-focus);
            outline-offset: -2px;
        }

        .actions {
            display: flex;
            flex-shrink: 0;
            padding-right: 0.25rem;
        }

        .alerted {
            color: var(--cds-support-error);
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
            overflow: hidden;
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
            text-overflow: ellipsis;
            white-space: nowrap;
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
        const selected = session.id === this.selected;
        return html`
            <li class=${selected ? 'selected' : ''}>
                <button class="session" type="button" aria-current=${selected ? 'true' : 'false'}
                    @click=${() => this.dispatchEvent(new CustomEvent(SessionList.eventSelect, { detail: session }))}>
                    <span class="model">${session.title || session.model || 'No model'}${session.event ? html` <cds-tag size="sm" type="purple">event</cds-tag>` : nothing}</span>
                    <span class="secondary" title=${formatTime(when)}>
                        ${session.title && session.model ? html`${session.model} · ` : nothing}${status ? html`<span class="status">${status}</span>` : formatAgo(when)}
                        ${tokens ? html` · ${tokens.toLocaleString()} tokens` : nothing}
                    </span>
                </button>
                ${selected && !session.event ? this.renderActions(session) : nothing}
            </li>
        `;
    }

    // renderActions returns the actions of the session which is open: its
    // events, which are red as one was just created, editing it, and
    // deleting it
    private renderActions(session: Session) {
        const action = (name: string, label: string, icon: unknown, alerted = false) => html`
            <cds-icon-button kind="ghost" size="sm" align="bottom-end"
                @click=${() => this.dispatchEvent(new CustomEvent(name, { detail: session }))}>
                <span slot="icon" class=${alerted ? 'alerted' : ''}>${iconLoader(icon as never)}</span>
                <span slot="tooltip-content">${label}</span>
            </cds-icon-button>
        `;
        return html`
            <div class="actions">
                ${action(SessionList.eventEvents, 'Events', Notification16, this.alerted)}
                ${action(SessionList.eventEdit, 'Edit chat', Edit16)}
                ${action(SessionList.eventDelete, 'Delete chat', TrashCan16)}
            </div>
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-session-list': SessionList;
    }
}
