// The page of a workspace: its header, with its title, what it has, and
// what it does, and its resources, in a table, which fills the page, or,
// when it's a tree, its folders, as a tree, beside the files of the folder
// which is shown

import { LitElement, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import type { ListController } from '@mutablelogic/go-server/list';
import '@mutablelogic/go-server/list/view';
import type { Column } from '@mutablelogic/go-server/list/view';
import '@mutablelogic/go-server/page/view';
import type { PanelController } from '@mutablelogic/go-server/panel';
import '@mutablelogic/go-server/panel/view';
import Folder16 from '@carbon/icons/es/folder/16.js';
import Document16 from '@carbon/icons/es/document/16.js';
import FolderDetails32 from '@carbon/icons/es/folder--details/32.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import '@carbon/web-components/es/components/tag/index.js';
import '../component/folder-tree';
import '../component/markdown-view';
import '@carbon/web-components/es/components/loading/index.js';
import { formatSize, formatWhen } from '../format';
import { routeHref } from '../router';
import { isFolder, type ResourceMeta, type Workspace } from '../schema/workspace';
import type { MarkdownPart } from '../api/workspace';

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

    // The workspace's folders, by the folder they're in, as they're loaded,
    // with their paths, which are shown as a tree, beside the files of the
    // folder which is shown, when it's a tree, and the function which loads
    // the folders of a folder, which is expanded
    @property({ attribute: false })
    folders = new Map<string, ResourceMeta[]>();

    expand?: (folder: string) => void;

    @property({ type: Boolean, reflect: true })
    tree = false;

    // The panel of a file, and the file it shows
    @property({ attribute: false })
    panel?: PanelController<FileDetail>;

    @property({ attribute: false })
    file?: FileDetail;

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
            ${this.missing ? html`
                <div class="content"><p>There's no workspace ${this.name}, or it isn't available now.</p></div>
            ` : this.tree ? html`
                <div class="workspace-split">
                    <nav class="workspace-tree" aria-label="Folders">
                        <llm-folder-tree .label=${workspace?.title || this.name} .folders=${this.folders} .selected=${this.path.join('/')}
                            @llm-folder-select=${this.selected} @llm-folder-expand=${this.expanded}></llm-folder-tree>
                    </nav>
                    <div class="workspace-files">
                        <gs-table .controller=${this.controller} .columns=${this.columns}
                            .itemKey=${(item: ResourceMeta) => item.uri}
                            empty="This folder has no files"></gs-table>
                    </div>
                </div>
            ` : html`
                <div class="workspace-files">
                    <gs-table .controller=${this.controller} .columns=${this.columns}
                        .itemKey=${(item: ResourceMeta) => item.uri}
                        empty="This workspace has no resources"></gs-table>
                </div>
            `}
            <gs-panel .controller=${this.panel}>${this.fileBody()}</gs-panel>
        `;
    }

    // fileBody renders the file which the panel shows: a preview, when it's
    // an image, what it is, its metadata, and its artwork
    private fileBody(): unknown {
        const file = this.file;
        if (!file) {
            return nothing;
        }
        const { item, url, artwork, loading, markdown } = file;
        // Its title, and type, are the panel's header, so its name is only
        // said when its title is another
        const details: [string, unknown][] = [
            ['Name', item.title && item.title !== item.name ? item.name : undefined],
            ['Path', item.path],
            ['Size', item.size === undefined ? undefined : formatSize(item.size)],
            ['Modified', item.modified ? formatWhen(item.modified) : undefined],
            ['URI', item.uri],
        ];
        return html`
            <div class="workspace-file">
                ${item.type?.startsWith('image/') ? html`<img class="preview" src=${url} alt=${item.title || item.name}>` : nothing}
                ${item.description ? html`<p>${item.description}</p>` : nothing}
                <dl>
                    ${details.filter(([, value]) => value !== undefined && value !== '').map(([term, value]) => html`<dt>${term}</dt><dd>${value}</dd>`)}
                </dl>
                ${item.meta?.length ? html`
                    <h3>Metadata</h3>
                    <dl>
                        ${item.meta.map((meta) => html`<dt>${meta.ns}:${meta.key}</dt><dd>${meta.text}</dd>`)}
                    </dl>
                ` : nothing}
                ${loading ? html`<cds-loading small inactive-text="Loading" class="preview-loading"></cds-loading>` : nothing}
                ${markdown?.body.trim() ? html`
                    <h3>Preview</h3>
                    <llm-markdown-view class="preview-markdown" .text=${markdown.body}></llm-markdown-view>
                    ${markdown.more ? html`<p class="preview-more">The rest isn't shown. Open the file for all of it.</p>` : nothing}
                ` : nothing}
                ${artwork.length ? html`
                    <h3>Artwork</h3>
                    <div class="artwork">
                        ${artwork.map((art) => html`<figure><img src=${art.url} alt=${art.key}><figcaption>${art.key}</figcaption></figure>`)}
                    </div>
                ` : nothing}
            </div>
        `;
    }

    // selected shows the folder which is selected in the tree
    private readonly selected = (event: Event) => {
        const path = (event as CustomEvent<string>).detail;
        if (path !== this.path.join('/')) {
            location.hash = routeHref('workspace', this.name, ...(path ? path.split('/') : []));
        }
    };

    // expanded loads the folders of a folder which is expanded in the tree
    private readonly expanded = (event: Event) => {
        this.expand?.((event as CustomEvent<string>).detail);
    };

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
                ${workspace.tree ? html`<cds-tag size="sm" type="purple">Tree</cds-tag>` : nothing}
            </span>
        `;
    }
}

// FileDetail is a file which the panel shows, with the URLs of its content,
// and its artwork
export interface FileDetail {
    item: ResourceMeta;
    url: string;
    artwork: { key: string; type?: string; url: string }[];
    loading?: boolean;
    markdown?: MarkdownPart;
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-workspace-page': WorkspacePage;
    }
}
