// A row of tiles, such as one for every model, and one for each provider,
// each with a label, a badge, such as a provider's type, and a count, which
// is less prominent when it's none, or isn't known. Clicking a tile fires an
// event with its value, so only what it counts is shown, or everything,
// when it has none, and the tile which is chosen is pressed, as a summary of
// a task queue's tasks, by their statuses, can be.

import { LitElement, css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';

// Carbon components
import '@carbon/web-components/es/components/skeleton-placeholder/index.js';
import '@carbon/web-components/es/components/tag/tag.js';

// CountTile is a tile: its value, which is undefined for everything, its
// label, its badge, its count, which is undefined when it isn't known, what
// it is, which is shown when it's hovered, and why what it counts can't be
// reached, when it can't, which is shown when it's hovered, with a word in
// place of its count, which is Unreachable, unless it's another, such as
// Unauthorized
export interface CountTile {
    value?: string;
    label: string;
    badge?: string;
    count?: number;
    title?: string;
    error?: string;
    status?: string;
    // Its logo, before its label: a URL of an image, or an icon, such as
    // Carbon's
    icon?: string | unknown;
}

@customElement('llm-count-filter')
export class CountFilter extends LitElement {
    // The name of the event fired when a tile is clicked, whose detail is
    // its value, or undefined, for everything
    static readonly eventSelect = 'llm-count-select';

    // The tiles, which are undefined while they're loading
    @property({ attribute: false })
    tiles?: CountTile[];

    // The value of the tile which is chosen, or none, for everything
    @property({ attribute: false })
    selected?: string;

    // What the tiles filter, such as Filter by provider, for assistive
    // technology
    @property()
    label = 'Filter';

    static styles = css`
        :host {
            display: block;
            margin: 0 0 1rem;
        }

        .summary {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(10rem, 1fr));
            gap: 1px;
        }

        button {
            all: unset;
            box-sizing: border-box;
            display: flex;
            flex-direction: column;
            gap: 0.25rem;
            padding: 0.75rem 1rem;
            background: var(--cds-layer-01, #f4f4f4);
            border-bottom: 2px solid transparent;
            cursor: pointer;
        }

        button:hover {
            background: var(--cds-layer-hover-01, #e8e8e8);
        }

        button:focus-visible {
            outline: 2px solid var(--cds-focus, #0f62fe);
            outline-offset: -2px;
        }

        button[aria-pressed='true'] {
            background: var(--cds-layer-selected-01, #e0e0e0);
            border-bottom-color: var(--cds-border-interactive, #0f62fe);
        }

        /* A tile's label, with its badge beside it */
        .label {
            display: flex;
            align-items: center;
            gap: 0.25rem;
            min-height: 1.5rem;
            color: var(--cds-text-secondary);
            font-size: var(--cds-label-01-font-size, 0.75rem);
        }

        .name {
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        .label cds-tag {
            margin: 0;
        }

        /* Its logo, on a small white tile, so a dark one is seen in the
           dark theme too, or an icon */
        .label img.brand {
            flex: none;
            box-sizing: border-box;
            width: 1rem;
            height: 1rem;
            padding: 1px;
            border-radius: 3px;
            background: #fff;
            object-fit: contain;
        }

        .label .icon {
            flex: none;
            display: inline-flex;
            fill: currentColor;
        }

        .count {
            font-size: var(--cds-heading-03-font-size, 1.25rem);
            font-variant-numeric: tabular-nums;
        }

        .zero {
            color: var(--cds-text-secondary);
        }

        /* What can't be reached is an error, in place of its count */
        .count.error {
            color: var(--cds-text-error);
            font-size: var(--cds-body-compact-01-font-size, 0.875rem);
            line-height: 1.75rem;
        }

        cds-skeleton-placeholder {
            width: 100%;
            height: 4.5rem;
        }
    `;

    render() {
        if (!this.tiles) {
            return html`<cds-skeleton-placeholder></cds-skeleton-placeholder>`;
        }
        return html`
            <div class="summary" role="group" aria-label=${this.label}>
                ${this.tiles.map((tile) => this.renderTile(tile))}
            </div>
        `;
    }

    private renderTile(tile: CountTile) {
        const pressed = tile.value === this.selected;

        // The badge of the tile which is chosen is blue, as its border is,
        // as gray is the tile's colour then
        return html`
            <button type="button" aria-pressed=${pressed ? 'true' : 'false'} title=${tile.error ?? tile.title ?? nothing}
                @click=${() => !pressed && this.dispatchEvent(new CustomEvent(CountFilter.eventSelect, { detail: tile.value }))}>
                <span class="label">${tile.icon === undefined ? nothing : typeof tile.icon === 'string' ? html`<img class="brand" src=${tile.icon} alt="">` : html`<span class="icon">${tile.icon}</span>`}<span class="name">${tile.label}</span>${tile.badge ? html`<cds-tag size="sm" type=${pressed ? 'blue' : 'cool-gray'}>${tile.badge}</cds-tag>` : nothing}</span>
                ${tile.error
                    ? html`<span class="count error">${tile.status ?? 'Unreachable'}</span>`
                    : html`<span class="count ${tile.count ? '' : 'zero'}">${tile.count?.toLocaleString() ?? '–'}</span>`}
            </button>
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-count-filter': CountFilter;
    }
}
