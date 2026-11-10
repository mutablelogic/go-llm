// A panel on the right of the page, which shows a chat's heartbeats, those
// which fire soonest first: what each is for, its instruction, when it
// fires, in words, and when it fires next, each with an action which deletes it, and
// beneath them the outcomes of the events for the chat, such as heartbeats
// which fired, the most recent first: when each fired, what it's for,
// what's become of it, how important it is, and its message, as Markdown

import { css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { Panel } from './panel';
import { markdown, markdownStyles } from './markdown';
import { describeSchedule, formatTime } from '../format';
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

    static styles = [Panel.styles, markdownStyles, css`
        /* The panel is wider than others, for the messages */
        .panel {
            width: min(40rem, 100vw);
        }

        table {
            width: 100%;
            border-collapse: collapse;
            font-size: 0.875rem;
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
            white-space: nowrap;
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
        }

        /* A reminder's schedule, under when it fires next, wraps, so the
           column is as narrow as the time is */
        td.when .schedule {
            margin-top: 0.25rem;
            white-space: normal;
            max-width: 9rem;
        }

        td.status cds-tag {
            display: block;
            margin: 0 0 0.25rem;
        }

        .title {
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
            width: 1%;
            padding: 0.25rem 0;
            vertical-align: middle;
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

    protected renderTitle() {
        return 'Events';
    }

    protected renderSubtitle() {
        return 'The events your assistant runs on schedule';
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

    // renderHeartbeats returns the chat's heartbeats, each with when it
    // fires, and when it fires next, or nothing, when the server has none
    private renderHeartbeats() {
        if (!this.heartbeats) {
            return nothing;
        }
        return html`
            <section>
                <h3>Reminders</h3>
                ${this.heartbeats.length === 0 ? html`<p class="empty">None are scheduled. Ask in the conversation for a reminder, or a briefing, to schedule one.</p>` : html`
                    <table>
                        <thead><tr><th>When</th><th>Reminder</th><th>Status</th><th></th></tr></thead>
                        <tbody>${this.heartbeats.map((heartbeat) => html`
                            <tr>
                                <td class="when">
                                    <div>${heartbeat.next ? formatTime(heartbeat.next) : heartbeat.fired ? `Fired ${formatTime(heartbeat.fired)}` : ''}</div>
                                    <div class="schedule">
                                        ${heartbeat.schedule
                                            ? html`<span title=${heartbeat.schedule}>${describeSchedule(heartbeat.schedule)}</span>${heartbeat.timezone ? html` (${heartbeat.timezone})` : nothing}`
                                            : html`Once`}
                                    </div>
                                </td>
                                <td>
                                    <div class="title">${heartbeat.title}</div>
                                    <div class="secondary">${heartbeat.prompt}</div>
                                </td>
                                <td class="status">
                                    <cds-tag size="sm" type=${reminderStatus(heartbeat)[1]}>${reminderStatus(heartbeat)[0]}</cds-tag>
                                </td>
                                <td class="action">
                                    <cds-icon-button kind="ghost" size="sm" align="left" ?disabled=${this.deleting.has(heartbeat.id)}
                                        @click=${() => this.dispatchEvent(new CustomEvent(EventView.eventDeleteHeartbeat, { detail: heartbeat }))}>
                                        ${iconLoader(TrashCan16, { slot: 'icon' })}
                                        <span slot="tooltip-content">Delete reminder</span>
                                    </cds-icon-button>
                                </td>
                            </tr>
                        `)}</tbody>
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
            return html`<p class="empty">No events yet. When a reminder scheduled in this conversation fires, your assistant's reply is shown here.</p>`;
        }
        return html`
            <table>
                <thead><tr><th>When</th><th>Message</th><th>Status</th></tr></thead>
                <tbody>${this.outcomes.map((outcome) => html`
                    <tr>
                        <td class="when">${formatTime(outcome.fired || outcome.created)}</td>
                        <td>
                            ${outcome.title ? html`<div class="title">${outcome.title}</div>` : nothing}
                            <div class="markdown">${markdown(outcome.message ?? '')}</div>
                        </td>
                        <td class="status">
                            ${outcome.status ? html`<cds-tag size="sm" type=${statusTypes[outcome.status] ?? 'gray'}>${outcome.status}</cds-tag>` : nothing}
                            ${outcome.importance ? html`<cds-tag size="sm" type=${importanceTypes[outcome.importance] ?? 'gray'}>${outcome.importance}</cds-tag>` : nothing}
                        </td>
                    </tr>
                `)}</tbody>
            </table>
            ${this.count > this.outcomes.length ? html`<p class="count">Showing the latest ${this.outcomes.length} of ${this.count}</p>` : nothing}
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-event-view': EventView;
    }
}
