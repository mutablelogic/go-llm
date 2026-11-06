// A panel on the right of the page, which shows the outcomes of the events
// for a chat, such as heartbeats which fired, the most recent first: when
// each fired, what it's for, what's become of it, how important it is, and
// its message, as Markdown

import { css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { Panel } from './panel';
import { markdown, markdownStyles } from './markdown';
import { formatTime } from '../format';
import type { EventOutcome } from '../schema/outcome';

// Carbon components
import '@carbon/web-components/es/components/tag/tag.js';
import '@carbon/web-components/es/components/skeleton-text/index.js';

// The colour of the tag of each status, and importance
const statusTypes: Record<string, string> = {
    scheduled: 'cyan',
    failed: 'red',
    pending: 'blue',
    notified: 'purple',
    delivered: 'green',
    ignored: 'cool-gray',
};
const importanceTypes: Record<string, string> = {
    alert: 'red',
    notify: 'teal',
    ignore: 'cool-gray',
};

@customElement('llm-event-view')
export class EventView extends Panel<string> {
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
        return 'What heartbeats, and other events, for this chat replied, the most recent first';
    }

    protected renderBody() {
        if (this.error) {
            return html`<p>${this.error}</p>`;
        } else if (!this.outcomes) {
            return html`<cds-skeleton-text paragraph line-count="4"></cds-skeleton-text>`;
        } else if (this.outcomes.length === 0) {
            return html`<p>No events yet. When a heartbeat scheduled in this chat fires, what it replied is shown here.</p>`;
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
