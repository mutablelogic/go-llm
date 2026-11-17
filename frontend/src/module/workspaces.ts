// The Workspaces module: each workspace, such as a connector's, the
// memory, or a folder of files, in the sidebar's Workspaces menu, and its
// folders, and resources, a folder at a time, in a page of their own, at
// #workspace/<name>/<folder>/…

import { ListController, type Page, type Sort } from '@mutablelogic/go-server/list';
import { PanelController, type PanelAction } from '@mutablelogic/go-server/panel';
import { MediaPlayer } from '../component/media-player';
import { contentURL, listFolder, listWorkspaces, readMarkdown } from '../api/workspace';
import { WorkspacePage, type FileDetail } from '../page/workspace-page';
import { routeHref } from '../router';
import { FolderType, isFolder, type ResourceMeta, type Workspace } from '../schema/workspace';
import { addPage, Section, type App } from './app';

// MaxPreview is the most bytes of a file's Markdown which are shown in its
// panel
const MaxPreview = 64 * 1024;

// MaxListed is the most folders, and resources, of a folder which are
// listed, as they're sorted, and paged, in the browser, as a workspace's
// pages are by cursor
const MaxListed = 5000;

// workspacesModule adds the workspaces' page to the app, and each workspace
// to the sidebar, as they're added, and removed
// WorkspacesModule is what the other modules use of the Workspaces module
export interface WorkspacesModule {
    // openResource shows a workspace's resource, by its URI, such as
    // resource://files/Movies/Blade%20Runner.mp4, in its folder, in its panel
    openResource(uri: string): void;
}

// eventOpen is fired, with a workspace's resource's URI, by a link to it,
// such as in a chat, to show it in its panel
export const eventOpen = 'llm-workspace-open';

export function workspacesModule(app: App): WorkspacesModule {
    const page = addPage<WorkspacePage>(app, 'llm-workspace-page', 'workspace-page', '');
    let workspaces: Workspace[] = [];

    // The folder which is shown, which is listed whole, and then sorted,
    // folders first, and paged
    let name = '';
    let path: string[] = [];
    // The folder is read once, and its snapshot paged, and sorted, without
    // reading it again, until it's another folder, or it's listed again with
    // the same page, and sort, which is the controller refreshing it, as it
    // changed, or the server can be reached again
    let snapshot: { key: string; items: ResourceMeta[] } | undefined;
    let lastList = '';
    const controller = new ListController<ResourceMeta>({
        name: 'Files',
        list: async (offset, limit, sort) => {
            if (!name) {
                return { count: 0, body: [] };
            }
            const key = `${name}/${path.join('/')}`;
            const call = JSON.stringify([key, offset, limit, sort]);
            const again = call === lastList;
            lastList = call;
            if (!snapshot || snapshot.key !== key || again) {
                const { items, more } = await listFolder(name, path.join('/'), MaxListed);
                if (more) {
                    app.notifications.push({ kind: 'warning', title: `Only the first ${MaxListed} are listed`, subtitle: 'The folder has more' });
                }
                snapshot = { key, items };
            }
            const items = snapshot.items;
            // With a tree, the folders are in it, and the files beside it
            return pageOf(page.tree ? items.filter((item) => !isFolder(item)) : items, offset, limit, sort);
        },
        same: (a, b) => a.uri === b.uri,
        notifications: app.notifications,
        changes: app.changes,
        filter: { schema: 'workspace', table: 'resource' },
        sort: { key: 'name', direction: 'ascending' },
        pageSize: 100,
    });
    page.controller = controller;

    // A file is shown in a panel: what it is, its metadata, and its
    // artwork, and its content is opened in another tab, downloaded, or, when
    // it's audio, or video, played in the sidebar's player
    const panel = new PanelController<FileDetail>();
    page.panel = panel;
    const showFile = async (item: ResourceMeta) => {
        const workspace = name;
        const [url, download, artwork] = await Promise.all([
            contentURL(workspace, item.uri),
            contentURL(workspace, item.uri, true),
            Promise.all((item.artwork ?? []).map(async (art) => ({ ...art, url: await contentURL(workspace, art.uri) }))),
        ]);
        if (workspace !== name) {
            return;
        }
        const title = item.title || item.name;
        const playable = /^(audio|video)\//.test(item.type ?? '');
        const actions: PanelAction[] = [];
        if (playable) {
            actions.push({
                label: 'Play',
                kind: 'primary',
                close: false,
                run: () => document.dispatchEvent(new CustomEvent(MediaPlayer.eventPlay, { detail: { url, name: title, type: item.type } })),
            });
        }
        actions.push(
            { label: 'Open', kind: playable ? 'secondary' : 'primary', close: false, run: () => window.open(url, '_blank', 'noopener') },
            { label: 'Download', kind: 'secondary', close: false, run: () => save(download, item.name) },
        );
        panel.setHeader({ title, subtitle: item.type });
        panel.setActions(actions);
        // Its Markdown, which is read once it's shown, unless it's an
        // image, audio, or video, which are shown, or played, as they are
        const previewed = !/^(image|audio|video)\//.test(item.type ?? '');
        const file = { item, url, artwork, loading: previewed };
        page.file = file;
        panel.open(file);
        if (previewed) {
            let markdown;
            try {
                markdown = await readMarkdown(workspace, item.uri, MaxPreview);
            } catch {
                // The file is shown without its Markdown
            }
            if (page.file === file) {
                page.file = { ...file, loading: false, markdown };
            }
        }
    };

    // A folder which is opened is shown, and a file in the panel
    controller.subscribe((state) => {
        const item = state.selected;
        if (!item) {
            return;
        }
        controller.close();
        if (isFolder(item)) {
            location.hash = routeHref('workspace', name, ...path, item.name);
        } else {
            void showFile(item);
        }
    });

    // The workspaces, in the sidebar's Workspaces menu, as they're added,
    // and removed, and the one which is shown
    const refresh = async () => {
        try {
            workspaces = (await listWorkspaces()).body ?? [];
        } catch (error) {
            app.notifications.error("The workspaces couldn't be loaded", error);
            return;
        }
        app.nav.setModule('workspaces', Section, workspaces.map((workspace) => ({
            id: `workspace/${workspace.name}`,
            label: workspace.title || workspace.name,
            href: routeHref('workspace', workspace.name),
            menu: 'Workspaces',
        })));
        show();
    };
    // The folders of a workspace which is a tree, by the folder they're in,
    // which are loaded a folder at a time: the top, and those of the folder
    // which is shown, and of each folder it's in, and of a folder which is
    // expanded. When its top has any, they're shown as a tree, beside the
    // files of the folder which is shown, or else its folders, and files,
    // are listed together. Those which are loaded are loaded again as they
    // change.
    let foldersOf = '';
    let folders = new Map<string, ResourceMeta[]>();
    const loadFolder = async (folder: string) => {
        const workspace = foldersOf;
        try {
            const { items } = await listFolder(workspace, folder, MaxListed, { type: FolderType });
            if (workspace !== foldersOf) {
                return;
            }
            folders = new Map(folders).set(folder, items);
            page.folders = folders;
            if (folder === '' && page.tree !== items.length > 0) {
                page.tree = items.length > 0;
                void controller.refresh();
            }
        } catch (error) {
            app.notifications.error("The folders couldn't be loaded", error);
        }
    };
    const loadFolders = (all: boolean) => {
        const wanted = page.workspace?.tree ? name : '';
        if (wanted !== foldersOf) {
            foldersOf = wanted;
            folders = new Map();
            page.folders = folders;
            page.tree = false;
        }
        if (!wanted) {
            return;
        }
        // The top, and each folder the folder which is shown is in, and it,
        // or, as they change, every one which is loaded
        const wantedFolders = new Set(path.map((_, i) => path.slice(0, i + 1).join('/')));
        wantedFolders.add('');
        for (const folder of all ? folders.keys() : wantedFolders) {
            if (all || !folders.has(folder)) {
                void loadFolder(folder);
            }
        }
    };
    page.expand = (folder) => {
        if (!folders.has(folder)) {
            void loadFolder(folder);
        }
    };
    app.changes.on({ schema: 'workspace', table: 'resource' }, () => loadFolders(true));

    const show = () => {
        page.workspace = workspaces.find((workspace) => workspace.name === name);
        page.missing = !!name && !page.workspace;
        loadFolders(false);

        // Its page is active in the sidebar, once it's there, as the
        // workspaces can be loaded after it's routed to
        if (page.workspace && !page.hidden) {
            app.nav.setActive(`workspace/${name}`);
        }
    };
    // As they're added, and removed, and as a connector's is available, or
    // isn't, as it's connected, and disconnected, and says its resources
    // changed, as it's ready
    app.changes.on([
        { schema: 'workspace', table: 'workspace' },
        { schema: 'workspace', table: 'resource' },
        { schema: 'mcp', table: 'connector' },
    ], () => void refresh());

    // Its route is the workspace, and the folder, which is listed again when
    // it changes
    app.router.add('workspace', page, (route) => {
        const [next = '', ...folders] = route.params;
        const changed = next !== name || folders.join('/') !== path.join('/');
        name = next;
        path = folders;
        page.name = name;
        page.path = path;
        show();
        if (changed) {
            panel.close();
            void controller.reset();
        }
        void openPending();
    });

    // A resource which is opened, by its URI, is shown in its folder, and
    // then in the panel, once the folder is, as it's routed to
    let pending: string | undefined;
    const openPending = async () => {
        const uri = pending;
        pending = undefined;
        if (!uri || !name) {
            return;
        }
        // What it is, from its folder, when it's there, or else its URI
        let item: ResourceMeta | undefined;
        try {
            const { items } = await listFolder(name, path.join('/'), MaxListed);
            item = items.find((candidate) => candidate.uri === uri);
        } catch {
            // It's shown by its URI
        }
        const base = uri.replace(/\/+$/, '').split('/').pop() ?? uri;
        let label = base;
        try {
            label = decodeURIComponent(base);
        } catch {
            // As it is
        }
        if (!item || !isFolder(item)) {
            void showFile(item ?? { uri, name: label });
        }
    };
    return {
        openResource(uri: string) {
            const match = uri.trim().match(/^resource:\/\/([^/]+)\/(.+)$/);
            if (!match) {
                return;
            }
            const workspace = decodeURIComponent(match[1]);
            let rest = match[2];
            try {
                rest = decodeURIComponent(rest);
            } catch {
                // As it is
            }
            // Its folder is its path's, under its workspace's root, or, for
            // its own URI, which isn't a path, the top
            const folders = rest.includes('://') ? [] : rest.split('/').slice(0, -1);
            pending = uri.trim();
            if (!page.hidden && workspace === name && folders.join('/') === path.join('/')) {
                void openPending();
            } else {
                location.hash = routeHref('workspace', workspace, ...folders);
            }
        },
    };
}

// save saves the content at the URL, which is an attachment, as the file
// with the name
function save(url: string, name: string) {
    const link = document.createElement('a');
    link.href = url;
    link.download = name;
    link.click();
}

// pageOf returns the page of the items, at the offset, of at most the limit,
// sorted, folders first, by their names, sizes, or when they were modified
function pageOf(items: ResourceMeta[], offset: number, limit: number, sort?: Sort): Page<ResourceMeta> {
    const direction = sort?.direction === 'descending' ? -1 : 1;
    const byName = (a: ResourceMeta, b: ResourceMeta) => (a.title || a.name).localeCompare(b.title || b.name, undefined, { numeric: true, sensitivity: 'base' });
    const compare = (a: ResourceMeta, b: ResourceMeta): number => {
        switch (sort?.key) {
            case 'size':
                return ((a.size ?? 0) - (b.size ?? 0)) || byName(a, b);
            case 'modified':
                return (Date.parse(a.modified ?? '') || 0) - (Date.parse(b.modified ?? '') || 0) || byName(a, b);
            default:
                return byName(a, b);
        }
    };
    const sorted = [...items].sort((a, b) => (Number(isFolder(b)) - Number(isFolder(a))) || direction * compare(a, b));
    return { count: sorted.length, body: sorted.slice(offset, offset + limit) };
}
