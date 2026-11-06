// A panel which shows the resources of the chat which is open, which it calls
// files, as the UI does: the files
// which were uploaded to it, and those its tools returned, such as speech,
// the most recently used first. A resource which is clicked is attached to
// the next message, and files which are dropped on the panel, or chosen,
// are uploaded to the chat, and attached. Audio, and video, are played in
// the player. It fires events to attach a resource, to upload files, to
// play a resource, to download its content, and to delete one.

import { css, html, nothing } from 'lit';
import { customElement, property, query } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Upload16 from '@carbon/icons/es/upload/16.js';
import Download16 from '@carbon/icons/es/download/16.js';
import PlayFilledAlt16 from '@carbon/icons/es/play--filled--alt/16.js';
import { playable } from './media-player';
import TrashCan16 from '@carbon/icons/es/trash-can/16.js';
import { Panel } from './panel';
import { resourceName, type Resource } from '../schema/resource';
import { formatAgo, formatSize } from '../format';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/icon-button/index.js';
import '@carbon/web-components/es/components/inline-loading/index.js';
import '@carbon/web-components/es/components/skeleton-text/index.js';

@customElement('llm-resource-panel')
export class ResourcePanel extends Panel<string> {
    // The names of the events fired to attach a resource, to play it, to
    // download its content, and to delete it, whose detail is the resource,
    // and to upload files, whose detail is them
    static readonly eventAttach = 'llm-resource-attach';
    static readonly eventPlay = 'llm-resource-play';
    static readonly eventDownload = 'llm-resource-download';
    static readonly eventDelete = 'llm-resource-delete';
    static readonly eventUpload = 'llm-resource-upload';

    // The session whose resources are shown, or none, when the panel is
    // closed
    @property({ attribute: false })
    session?: string;

    // Its resources, which are undefined while they're loading, and how many
    // there are
    @property({ attribute: false })
    resources?: Resource[];

    @property({ attribute: false })
    count = 0;

    // The hashes of the resources which are attached to the next message
    @property({ attribute: false })
    attached: string[] = [];

    // How many files are being uploaded
    @property({ attribute: false })
    uploading = 0;

    // Whether files are dragged over the panel
    @property({ type: Boolean, reflect: true })
    dragging = false;

    @query('input[type=file]')
    private picker?: HTMLInputElement;

    static styles = [Panel.styles, css`
        .drop {
            display: flex;
            flex-direction: column;
            align-items: center;
            gap: 0.75rem;
            margin-bottom: 1.5rem;
            padding: 1.5rem 1rem;
            border: 1px dashed var(--cds-border-strong-01);
            color: var(--cds-text-secondary);
            font-size: 0.875rem;
            text-align: center;
        }

        :host([dragging]) .drop {
            border: 2px dashed var(--cds-focus, #0f62fe);
            background: var(--cds-layer-hover-01);
        }

        input[type=file] {
            display: none;
        }

        ul {
            margin: 0;
            padding: 0;
            list-style: none;
        }

        li {
            display: flex;
            align-items: center;
            gap: 0.25rem;
            border-bottom: 1px solid var(--cds-border-subtle-01);
        }

        li button.item {
            flex: 1;
            min-width: 0;
            padding: 0.625rem 0.5rem;
            border: none;
            background: none;
            color: inherit;
            font: inherit;
            text-align: left;
            cursor: pointer;
        }

        li button.item:hover {
            background: var(--cds-layer-hover-01);
        }

        li button.item:focus-visible {
            outline: 2px solid var(--cds-focus, #0f62fe);
            outline-offset: -2px;
        }

        li.attached {
            background: var(--cds-layer-selected-01);
        }

        .name {
            display: block;
            overflow: hidden;
            font-size: 0.875rem;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        .meta {
            display: block;
            margin-top: 0.125rem;
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
        }

        .none {
            color: var(--cds-text-secondary);
            font-size: 0.875rem;
        }
    `];

    protected current() {
        return this.session;
    }

    protected renderTitle() {
        return 'Files';
    }

    protected renderSubtitle() {
        return 'Click one to attach it to your next message, or drop files to upload them';
    }

    protected renderBody() {
        return html`
            <div class="drop" @dragenter=${this.handleDrag} @dragover=${this.handleDrag}
                @dragleave=${this.handleDragLeave} @drop=${this.handleDrop}>
                <span>Drop files here, or</span>
                <cds-button kind="tertiary" size="sm" @click=${() => this.picker?.click()}>
                    Choose files ${iconLoader(Upload16, { slot: 'icon' })}
                </cds-button>
                <input type="file" multiple @change=${this.handleChoose}>
                ${this.uploading > 0
                    ? html`<cds-inline-loading status="active">Uploading ${this.uploading === 1 ? 'a file' : `${this.uploading} files`}…</cds-inline-loading>`
                    : nothing}
            </div>
            ${this.renderList()}
        `;
    }

    private renderList() {
        if (!this.resources) {
            return html`<cds-skeleton-text paragraph line-count="4"></cds-skeleton-text>`;
        } else if (this.resources.length === 0) {
            return html`<p class="none">This chat has no files yet.</p>`;
        }
        return html`
            <h3>${this.count === 1 ? '1 file' : `${this.count.toLocaleString()} files`}</h3>
            <ul>${this.resources.map((resource) => this.renderResource(resource))}</ul>
        `;
    }

    private renderResource(resource: Resource) {
        const attached = this.attached.includes(resource.hash);
        const meta = [resource.type, formatSize(resource.size), formatAgo(resource.used ?? resource.created)].filter(Boolean).join(' · ');
        return html`
            <li class=${attached ? 'attached' : ''}>
                <button class="item" aria-pressed=${attached ? 'true' : 'false'} title=${attached ? 'Detach it' : 'Attach it'}
                    @click=${() => this.fire(ResourcePanel.eventAttach, resource)}>
                    <span class="name">${resourceName(resource)}${attached ? ' (attached)' : ''}</span>
                    <span class="meta">${meta}</span>
                </button>
                ${playable(resource.type) ? html`
                <cds-icon-button kind="ghost" size="sm" align="left" @click=${() => this.fire(ResourcePanel.eventPlay, resource)}>
                    ${iconLoader(PlayFilledAlt16, { slot: 'icon' })}
                    <span slot="tooltip-content">Play</span>
                </cds-icon-button>` : nothing}
                <cds-icon-button kind="ghost" size="sm" align="left" @click=${() => this.fire(ResourcePanel.eventDownload, resource)}>
                    ${iconLoader(Download16, { slot: 'icon' })}
                    <span slot="tooltip-content">Download</span>
                </cds-icon-button>
                <cds-icon-button kind="ghost" size="sm" align="left" @click=${() => this.fire(ResourcePanel.eventDelete, resource)}>
                    ${iconLoader(TrashCan16, { slot: 'icon' })}
                    <span slot="tooltip-content">Delete</span>
                </cds-icon-button>
            </li>
        `;
    }

    private fire(name: string, detail: unknown) {
        this.dispatchEvent(new CustomEvent(name, { detail }));
    }

    // upload fires the event to upload the files, unless there are none
    private upload(files: File[]) {
        if (files.length > 0) {
            this.fire(ResourcePanel.eventUpload, files);
        }
    }

    private handleChoose() {
        this.upload(Array.from(this.picker?.files ?? []));
        if (this.picker) {
            this.picker.value = '';
        }
    }

    // handleDrag accepts files which are dragged over the drop zone
    private handleDrag(event: DragEvent) {
        if (!hasFiles(event)) {
            return;
        }
        event.preventDefault();
        if (event.dataTransfer) {
            event.dataTransfer.dropEffect = 'copy';
        }
        this.dragging = true;
    }

    private handleDragLeave(event: DragEvent) {
        // Leaving the drop zone for an element in it isn't leaving it
        if (!(event.relatedTarget instanceof Node && (event.currentTarget as Node).contains(event.relatedTarget))) {
            this.dragging = false;
        }
    }

    private handleDrop(event: DragEvent) {
        event.preventDefault();
        this.dragging = false;
        this.upload(Array.from(event.dataTransfer?.files ?? []));
    }
}

// hasFiles returns whether files are dragged
export function hasFiles(event: DragEvent): boolean {
    return Array.from(event.dataTransfer?.types ?? []).includes('Files');
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-resource-panel': ResourcePanel;
    }
}
