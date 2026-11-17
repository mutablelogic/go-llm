// A tree of folders, loaded a folder at a time: each folder which is shown
// is a row, the width of the tree, indented by how deep it is, with a caret
// which expands it, or collapses it, when it has folders, or they aren't
// loaded yet. The folder which is selected has a bar at the tree's left,
// and its row is highlighted.

import { LitElement, css, html, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import CaretDown16 from '@carbon/icons/es/caret--down/16.js';
import CaretRight16 from '@carbon/icons/es/caret--right/16.js';
import Folder16 from '@carbon/icons/es/folder/16.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import type { ResourceMeta } from '../schema/workspace';

@customElement('llm-folder-tree')
export class FolderTree extends LitElement {
    // eventSelect is fired, with the folder's path, when it's selected, and
    // eventExpand when it's expanded, so its folders are loaded
    static readonly eventSelect = 'llm-folder-select';
    static readonly eventExpand = 'llm-folder-expand';

    // The label of the top, whose path is empty
    @property()
    label = '';

    // The folders, by the path of the folder they're in, as they're loaded
    @property({ attribute: false })
    folders = new Map<string, ResourceMeta[]>();

    // The path of the folder which is selected, and each it's in is
    // expanded
    @property()
    selected = '';

    // The paths of the folders which are expanded
    @state()
    private expanded = new Set<string>(['']);

    static override styles = css`
        :host {
            display: block;
            font-size: var(--cds-body-compact-01-font-size, 0.875rem);
            line-height: var(--cds-body-compact-01-line-height, 1.28572);
            letter-spacing: var(--cds-body-compact-01-letter-spacing, 0.16px);
        }

        .row {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            min-height: 2rem;
            padding-right: 1rem;
            color: var(--cds-text-secondary, #525252);
            white-space: nowrap;
            cursor: pointer;
            user-select: none;
        }

        .row:hover {
            background: var(--cds-layer-hover-01, #e8e8e8);
            color: var(--cds-text-primary, #161616);
        }

        .row:focus-visible {
            outline: 2px solid var(--cds-focus, #0f62fe);
            outline-offset: -2px;
        }

        /* The folder which is selected: a bar at the tree's left, and its
           row highlighted, however deep it is */
        .row[aria-selected='true'] {
            background: var(--cds-layer-selected-01, #e0e0e0);
            box-shadow: inset 0.25rem 0 0 var(--cds-interactive, #0f62fe);
            color: var(--cds-text-primary, #161616);
        }

        .toggle,
        .icon {
            display: inline-flex;
            flex: none;
        }

        .toggle {
            width: 1rem;
        }

        svg {
            fill: currentColor;
        }

        .label {
            overflow: hidden;
            text-overflow: ellipsis;
        }
    `;

    protected override willUpdate(changed: Map<string, unknown>) {
        // The selected folder, and those it's in, are expanded, so it's
        // shown, with its folders
        if (changed.has('selected') && this.selected) {
            const parts = this.selected.split('/');
            const expanded = new Set(this.expanded);
            parts.forEach((_, i) => expanded.add(parts.slice(0, i + 1).join('/')));
            this.expanded = expanded;
        }
    }

    protected override render() {
        return html`<div role="tree" aria-label=${this.label}>${this.rows(this.label, '', 0)}</div>`;
    }

    // rows returns the folder's row, and, when it's expanded, its folders'
    private rows(name: string, path: string, depth: number): unknown[] {
        const children = this.folders.get(path);
        const expandable = children === undefined || children.length > 0;
        const open = expandable && this.expanded.has(path);
        const result: unknown[] = [html`
            <div class="row" role="treeitem" tabindex="0" title=${name}
                style="padding-left: ${0.75 + depth * 1.25}rem"
                aria-level=${depth + 1}
                aria-selected=${this.selected === path ? 'true' : 'false'}
                aria-expanded=${expandable ? String(open) : nothing}
                @click=${() => this.select(path)}
                @keydown=${(event: KeyboardEvent) => this.key(event, path, expandable, open)}>
                <span class="toggle" @click=${(event: Event) => {
                    event.stopPropagation();
                    this.toggle(path, !open);
                }}>${expandable ? iconLoader(open ? CaretDown16 : CaretRight16) : nothing}</span>
                <span class="icon">${iconLoader(Folder16)}</span>
                <span class="label">${name}</span>
            </div>
        `];
        if (open && children) {
            for (const folder of sorted(children)) {
                result.push(...this.rows(folder.title || folder.name, pathOf(folder, path), depth + 1));
            }
        }
        return result;
    }

    // select selects the folder, which is expanded
    private select(path: string) {
        this.toggle(path, true);
        this.dispatchEvent(new CustomEvent(FolderTree.eventSelect, { detail: path, bubbles: true, composed: true }));
    }

    // toggle expands the folder, which loads its folders, or collapses it
    private toggle(path: string, open: boolean) {
        const expanded = new Set(this.expanded);
        if (open) {
            expanded.add(path);
            this.dispatchEvent(new CustomEvent(FolderTree.eventExpand, { detail: path, bubbles: true, composed: true }));
        } else {
            expanded.delete(path);
        }
        this.expanded = expanded;
    }

    // key selects the folder with Enter, or Space, and expands it, or
    // collapses it, with the right, and left, arrows
    private key(event: KeyboardEvent, path: string, expandable: boolean, open: boolean) {
        switch (event.key) {
            case 'Enter':
            case ' ':
                event.preventDefault();
                this.select(path);
                break;
            case 'ArrowRight':
                if (expandable && !open) {
                    event.preventDefault();
                    this.toggle(path, true);
                }
                break;
            case 'ArrowLeft':
                if (open) {
                    event.preventDefault();
                    this.toggle(path, false);
                }
                break;
        }
    }
}

// sorted returns the folders, in order of their names
function sorted(folders: ResourceMeta[]): ResourceMeta[] {
    return [...folders].sort((a, b) => (a.title || a.name).localeCompare(b.title || b.name, undefined, { numeric: true, sensitivity: 'base' }));
}

// pathOf returns the folder's path, under the top, or, when it hasn't one,
// its name, in the folder it's in
function pathOf(folder: ResourceMeta, parent: string): string {
    const path = (folder.path ?? '').replace(/^\/+|\/+$/g, '');
    return path || (parent ? `${parent}/${folder.name}` : folder.name);
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-folder-tree': FolderTree;
    }
}
