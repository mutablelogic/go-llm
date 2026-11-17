// The page of a workspace: its header, with its title, what it has, and
// what it does, the folder which is shown, as a breadcrumb, and the folder's
// folders, and resources, in a table, which fills the page

import { LitElement, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import type { ListController } from '@mutablelogic/go-server/list';
import '@mutablelogic/go-server/list/view';
import type { Column } from '@mutablelogic/go-server/list/view';
import '@mutablelogic/go-server/page/view';
import Folder16 from '@carbon/icons/es/folder/16.js';
import Document16 from '@carbon/icons/es/document/16.js';
import FolderDetails32 from '@carbon/icons/es/folder--details/32.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import '@carbon/web-components/es/components/breadcrumb/index.js';
import '@carbon/web-components/es/components/tag/index.js';
import { formatSize, formatWhen } from '../format';
import { routeHref } from '../router';
import { isFolder, type ResourceMeta, type Workspace } from '../schema/workspace';

@customElement('llm-workspace-page')
export class WorkspacePage extends LitElement {
    // The workspace, which is undefined while it's loading, or when there's
    // none with the name
    @property({ attribute: false })
    workspace?: Workspace;

    // The name of the workspace, and the folders of the folder which is
    // shown, from its top
    @property()
    name = '';

    @property({ attribute: false })
    path: string[] = [];

    // Whether there's no workspace with the name
    @property({ type: Boolean })
    missing = false;

    // The folder's folders, and resources
    @property({ attribute: false })
    controller?: ListController<ResourceMeta>;

    // The columns of the table: the name, with whether it's a folder, its
    // type, size, and when it was modified, which are sorted by
    private readonly columns: Column<ResourceMeta>[] = [
        {
            header: 'Name',
            sort: 'name',
            sortFirst: 'ascending',
            cell: (item) => html`<span style="display: inline-flex; align-items: center; gap: 0.5rem">${iconLoader(isFolder(item) ? Folder16 : Document16)}<span>${item.title || item.name}</span></span>`,
        },
        { header: 'Type', cell: (item) => isFolder(item) ? 'Folder' : (item.type ?? '') },
        { header: 'Size', sort: 'size', cell: (item) => isFolder(item) || item.size === undefined ? '' : formatSize(item.size) },
        { header: 'Modified', sort: 'modified', cell: (item) => item.modified ? formatWhen(item.modified) : '' },
    ];

    // The page is in the light DOM, so the app's page styles, which fill the
    // screen, apply to it
    protected override createRenderRoot() {
        return this;
    }

    protected override render() {
        const workspace = this.workspace;
        return html`
            <gs-page-header style="--gs-page-header-margin: 0"
                .heading=${workspace?.title || this.name}
                .icon=${FolderDetails32}
                .details=${this.details(workspace)}></gs-page-header>
            <div class="content">
                ${this.missing ? html`<p>There's no workspace ${this.name}, or it isn't available now.</p>` : html`
                    <cds-breadcrumb no-trailing-slash class="workspace-path">
                        ${[this.name, ...this.path].map((folder, i, all) => html`
                            <cds-breadcrumb-item>
                                ${i === all.length - 1
                                    ? html`<cds-breadcrumb-link is-currentpage>${i === 0 ? (workspace?.title || folder) : folder}</cds-breadcrumb-link>`
                                    : html`<cds-breadcrumb-link href=${routeHref('workspace', this.name, ...this.path.slice(0, i))}>${i === 0 ? (workspace?.title || folder) : folder}</cds-breadcrumb-link>`}
                            </cds-breadcrumb-item>
                        `)}
                    </cds-breadcrumb>
                    <gs-table .controller=${this.controller} .columns=${this.columns}
                        .itemKey=${(item: ResourceMeta) => item.uri}
                        empty="This folder is empty"></gs-table>
                `}
            </div>
        `;
    }

    // details are what the workspace has, and what it does, as tags: whether
    // its resources are read, and whether its changes are followed now
    private details(workspace?: Workspace) {
        if (!workspace) {
            return nothing;
        }
        return html`
            ${workspace.description ? html`<span>${workspace.description}</span>` : nothing}
            <span class="workspace-tags">
                ${workspace.version ? html`<cds-tag size="sm" type="cool-gray">${workspace.version}</cds-tag>` : nothing}
                <cds-tag size="sm" type=${workspace.read ? 'green' : 'gray'}>${workspace.read ? 'Read' : 'Listed only'}</cds-tag>
                <cds-tag size="sm" type=${workspace.watch ? 'blue' : 'gray'}>${workspace.watch ? 'Watched' : 'Not watched'}</cds-tag>
            </span>
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-workspace-page': WorkspacePage;
    }
}
