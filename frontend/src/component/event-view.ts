// A panel on the right of the page, which shows a chat's heartbeats, those
// which fire soonest first: what each is for, its instruction, when it
// fires, in words, and when it fires next, each with an action which deletes it, and
// beneath them the outcomes of the events for the chat, such as heartbeats
// which fired, the most recent first: when each fired, what it's for,
// what's become of it, how important it is, and its message, as Markdown

import { css, html, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { Panel } from './panel';
import Notification16 from '@carbon/icons/es/notification/16.js';
import { markdown, markdownStyles } from './markdown';
import { describeSchedule, formatTime, formatWhen } from '../format';
import type { EventOutcome } from '../schema/outcome';
import type { Heartbeat } from '../schema/heartbeat';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import TrashCan16 from '@carbon/icons/es/trash-can/16.js';

// Carbon components
import '@carbon/web-components/es/components/tag/tag.js';
import '@carbon/web-components/es/components/skeleton-text/index.js';
import '@carbon/web-components/es/components/icon-button/index.js';

// The colour of the tag of each status, and importance
const statusTypes: Record<string, string> = {
    scheduled: 'cyan',
    failed: 'red',
    pending: 'blue',
    notified: 'purple',
    delivered: 'green',
    ignored: 'cool-gray',
};
// reminderStatus returns what's become of a reminder, and its tag's colour:
// recurring, on a schedule, upcoming, once, before it fires, or completed,
// once it has, and won't again
function reminderStatus(heartbeat: Heartbeat): [string, string] {
    if (heartbeat.schedule) {
        return ['Recurring', 'cyan'];
    } else if (heartbeat.next) {
        return ['Upcoming', 'blue'];
    }
    return ['Completed', 'green'];
}

// capitalized returns the word with its first letter in capitals, as a tag
// shows it
function capitalized(word: string): string {
    return word.charAt(0).toUpperCase() + word.slice(1);
}

// Long text is shown in part, until it's shown in full: a task's instruction
// of more than this many characters, and a reply of more, or with an image
const longPrompt = 160;
const longMessage = 400;

const importanceTypes: Record<string, string> = {
    alert: 'red',
    notify: 'teal',
    ignore: 'cool-gray',
};

@customElement('llm-event-view')
export class EventView extends Panel<string> {
    // The name of the event fired to delete a heartbeat, whose detail is the
    // heartbeat
    static readonly eventDeleteHeartbeat = 'llm-event-view-delete-heartbeat';

    // The session whose events are shown, which is undefined when the panel
    // is closed
    @property({ attribute: false })
    session?: string;

    // The outcomes of its events, the most recent first, which are undefined
    // while they're loading, and how many there are
    @property({ attribute: false })
    outcomes?: EventOutcome[];

    @property({ attribute: false })
    count = 0;

    // The chat's heartbeats, those which fire soonest first, which are
    // undefined while they're loading, or when the server has none, and the
    // identifiers of those which are being deleted
    @property({ attribute: false })
    heartbeats?: Heartbeat[];

    @property({ attribute: false })
    deleting: ReadonlySet<string> = new Set();

    // Why they couldn't be loaded, when they couldn't
    @property({ attribute: false })
    error?: string;

    // The tasks, and replies, by their identifiers, which are shown in full
    @state()
    private expanded: ReadonlySet<string> = new Set();

    static styles = [Panel.styles, markdownStyles, css`
        /* The panel is wider than others, for the messages */
        .panel {
            width: min(40rem, 100vw);
        }

        /* Both tables have the same columns, so they line up: when, what,
           its status, and an action */
        table {
            width: 100%;
            border-collapse: collapse;
            table-layout: fixed;
            font-size: 0.875rem;
        }

        col.when {
            width: 8.5rem;
        }

        col.status {
            width: 6.5rem;
        }

        col.action {
            width: 2.5rem;
        }

        th {
            text-align: left;
            font-weight: 600;
            padding: 0.5rem;
            border-bottom: 1px solid var(--cds-border-strong-01);
            white-space: nowrap;
        }

        td {
            padding: 0.75rem 0.5rem;
            border-bottom: 1px solid var(--cds-border-subtle-01);
            vertical-align: top;
        }

        td.when {
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
            line-height: 1.33;
        }

        td.when .schedule {
            margin-top: 0.25rem;
        }

        /* A task's instruction, which is read as the replies are */
        .prompt {
            color: var(--cds-text-secondary);
            line-height: 1.43;
            white-space: pre-line;
            overflow-wrap: anywhere;
        }

        /* Long text is shown in part, fading out, until it's shown in full */
        .clamped {
            max-height: 4.5rem;
            overflow: hidden;
            mask-image: linear-gradient(to bottom, black 60%, transparent);
        }

        .clamped.message {
            max-height: 9rem;
        }

        .more {
            padding: 0.25rem 0 0;
            border: none;
            background: none;
            font: inherit;
            font-size: 0.75rem;
            color: var(--cds-link-primary);
            cursor: pointer;
        }

        .more:hover {
            text-decoration: underline;
        }

        .markdown img {
            max-width: 8rem;
        }

        td.status cds-tag {
            display: block;
            margin: 0 0 0.25rem;
        }

        /* A row's title, which isn't .title, the panel's heading's */
        .row-title {
            font-weight: 600;
            margin-bottom: 0.25rem;
        }

        .markdown > :first-child {
            margin-top: 0;
        }

        .markdown > :last-child {
            margin-bottom: 0;
        }

        h3 {
            margin: 0 0 0.5rem;
            font-size: 0.875rem;
            font-weight: 600;
        }

        section + section {
            margin-top: 2rem;
        }

        /* What's said when there's nothing to show, or it couldn't be loaded,
           which is the same in each section */
        .empty {
            margin: 0;
            color: var(--cds-text-secondary);
            font-size: 0.875rem;
        }

        .secondary {
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
        }

        td.action {
            padding: 0.5rem 0;
            text-align: right;
        }

        .count {
            margin-top: 1rem;
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
        }
    `];

    protected current() {
        return this.session;
    }

    protected renderIcon() {
        return iconLoader(Notification16);
    }

    protected renderTitle() {
        return 'Tasks';
    }

    protected renderSubtitle() {
        return 'The tasks your assistant performs on schedule';
    }

    protected renderBody() {
        return html`
            ${this.renderHeartbeats()}
            <section>
                ${this.heartbeats ? html`<h3>What your assistant replied</h3>` : nothing}
                ${this.renderOutcomes()}
            </section>
        `;
    }

    // renderHeartbeats returns the chat's scheduled tasks, each with when it
    // fires next, its schedule, and its status, or nothing, when the server
    // has none
    private renderHeartbeats() {
        if (!this.heartbeats) {
            return nothing;
        }
        return html`
            <section>
                <h3>Scheduled tasks</h3>
                ${this.heartbeats.length === 0 ? html`<p class="empty">None are scheduled. Ask in the conversation for a reminder, or a briefing, to schedule one.</p>` : html`
                    <table>
                        ${this.columns()}
                        <thead><tr><th>When</th><th>Task</th><th>Status</th><th></th></tr></thead>
                        <tbody>${this.heartbeats.map((heartbeat) => {
                            const [status, type] = reminderStatus(heartbeat);
                            const when = heartbeat.next ?? heartbeat.fired;
                            return html`
                            <tr>
                                <td class="when">
                                    <div title=${formatTime(when)}>${formatWhen(when)}</div>
                                    <div class="schedule">
                                        ${heartbeat.schedule
                                            ? html`<span title=${heartbeat.schedule}>${describeSchedule(heartbeat.schedule)}</span>${heartbeat.timezone ? html` (${heartbeat.timezone})` : nothing}`
                                            : html`Once`}
                                    </div>
                                </td>
                                <td>
                                    <div class="row-title">${heartbeat.title}</div>
                                    ${this.collapsible(heartbeat.id, heartbeat.prompt.length > longPrompt, html`<div class="prompt">${heartbeat.prompt}</div>`)}
                                </td>
                                <td class="status"><cds-tag size="sm" type=${type}>${status}</cds-tag></td>
                                <td class="action">
                                    <cds-icon-button kind="ghost" size="sm" align="left" ?disabled=${this.deleting.has(heartbeat.id)}
                                        @click=${() => this.dispatchEvent(new CustomEvent(EventView.eventDeleteHeartbeat, { detail: heartbeat }))}>
                                        ${iconLoader(TrashCan16, { slot: 'icon' })}
                                        <span slot="tooltip-content">Delete task</span>
                                    </cds-icon-button>
                                </td>
                            </tr>
                        `;
                        })}</tbody>
                    </table>
                `}
            </section>
        `;
    }

    // renderOutcomes returns the outcomes of the chat's events, the most
    // recent first
    private renderOutcomes() {
        if (this.error) {
            return html`<p class="empty">${this.error}</p>`;
        } else if (!this.outcomes) {
            return html`<cds-skeleton-text paragraph line-count="4"></cds-skeleton-text>`;
        } else if (this.outcomes.length === 0) {
            return html`<p class="empty">No events yet. When a task scheduled in this conversation runs, your assistant's reply is shown here.</p>`;
        }
        return html`
            <table>
                ${this.columns()}
                <thead><tr><th>When</th><th>Message</th><th>Status</th><th></th></tr></thead>
                <tbody>${this.outcomes.map((outcome) => {
                    const when = outcome.fired || outcome.created;
                    const message = outcome.message ?? '';
                    return html`
                    <tr>
                        <td class="when" title=${formatTime(when)}>${formatWhen(when)}</td>
                        <td>
                            ${outcome.title ? html`<div class="row-title">${outcome.title}</div>` : nothing}
                            ${this.collapsible(outcome.id, message.length > longMessage || message.includes('!['), html`<div class="markdown">${markdown(message)}</div>`, 'message')}
                        </td>
                        <td class="status">
                            ${outcome.status ? html`<cds-tag size="sm" type=${statusTypes[outcome.status] ?? 'gray'}>${capitalized(outcome.status)}</cds-tag>` : nothing}
                            ${outcome.importance ? html`<cds-tag size="sm" type=${importanceTypes[outcome.importance] ?? 'gray'}>${capitalized(outcome.importance)}</cds-tag>` : nothing}
                        </td>
                        <td></td>
                    </tr>
                `;
                })}</tbody>
            </table>
            ${this.count > this.outcomes.length ? html`<p class="count">Showing the latest ${this.outcomes.length} of ${this.count}</p>` : nothing}
        `;
    }

    // columns returns the widths of the tables' columns, which are the same
    // in each, so they line up
    private columns() {
        return html`<colgroup><col class="when"><col><col class="status"><col class="action"></colgroup>`;
    }

    // collapsible returns the content, in part, with a link which shows it
    // in full, when it's long, until it's shown, or in full
    private collapsible(id: string, long: boolean, content: unknown, kind = '') {
        if (!long) {
            return content;
        }
        const expanded = this.expanded.has(id);
        return html`
            <div class=${expanded ? kind : `clamped ${kind}`}>${content}</div>
            <button class="more" type="button" aria-expanded=${String(expanded)} @click=${() => this.toggle(id)}>${expanded ? 'Show less' : 'Show more'}</button>
        `;
    }

    // toggle shows the content in full, or in part again
    private toggle(id: string) {
        const expanded = new Set(this.expanded);
        if (!expanded.delete(id)) {
            expanded.add(id);
        }
        this.expanded = expanded;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-event-view': EventView;
    }
}
