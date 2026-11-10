// The list of sessions, most recently changed first, each with its title,
// or its model, when it has none, its model, beneath its title, when it has
// one, when it was last changed, the tokens of its conversation, and whether
// someone is typing in it, or its model is replying. Clicking a session
// fires an event, so it's opened, and the session which is open is
// selected, with its actions on its right: its events, editing it, and
// deleting it, of which an event's session, which is created again when
// its event runs again, has only deleting it, and which isn't deleted while
// it's replying.

import { LitElement, css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { memoryWarning, sessionMemory, sessionMemoryPercent, type Session } from '../schema/session';
import { formatAgo, formatTime } from '../format';
import { MinuteController } from './minute';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import WarningAlt16 from '@carbon/icons/es/warning--alt/16.js';

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

    // The input token limits of the models, by their canonical names, so a
    // session says how full its memory is
    @property({ attribute: false })
    limits: ReadonlyMap<string, number> = new Map();

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

        .model {
            display: block;
            font-size: 0.875rem;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        /* A small mark that the memory is nearly full, in the text's colour,
           so it's seen, but isn't loud */
        .nearly-full svg {
            margin-left: 0.25rem;
            vertical-align: -3px;
            fill: currentColor;
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
            return html`<div class="empty">No conversations yet</div>`;
        }
        return html`<ul>${this.sessions.map((session) => this.renderSession(session))}</ul>`;
    }

    private renderSession(session: Session) {
        const limit = session.model ? this.limits.get(session.model) : undefined;
        const memory = sessionMemory(session, limit);
        const nearlyFull = (sessionMemoryPercent(session, limit) ?? 0) >= memoryWarning;
        const status = session.status === 'thinking' ? 'thinking…' : session.status === 'replying' ? 'replying…' : session.status === 'typing' ? 'typing…' : '';
        const when = session.modified ?? session.created;
        const selected = session.id === this.selected;
        return html`
            <li class=${selected ? 'selected' : ''}>
                <button class="session" type="button" aria-current=${selected ? 'true' : 'false'}
                    @click=${() => this.dispatchEvent(new CustomEvent(SessionList.eventSelect, { detail: session }))}>
                    <span class="model">${session.title || session.model || 'No model'}${session.event ? html` <cds-tag size="sm" type="purple">event</cds-tag>` : nothing}</span>
                    <span class="secondary" title=${formatTime(when)}>
                        ${session.title && session.model ? html`${session.model} · ` : nothing}${status ? html`<span class="status">${status}</span>` : formatAgo(when)}
                        ${memory ? html` · ${memory}` : nothing}${nearlyFull ? html`<span class="nearly-full" title="Its memory is nearly full, so its earlier messages are summarised soon">${iconLoader(WarningAlt16)}</span>` : nothing}
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
