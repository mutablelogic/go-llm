// The Workspaces module: each workspace, such as a connector's, the
// memory, or a folder of files, in the sidebar's Workspaces menu, and its
// folders, and resources, a folder at a time, in a page of their own, at
// #workspace/<name>/<folder>/…

import { ListController, type Page, type Sort } from '@mutablelogic/go-server/list';
import { contentURL, listFolder, listWorkspaces } from '../api/workspace';
import { WorkspacePage } from '../page/workspace-page';
import { routeHref } from '../router';
import { isFolder, type ResourceMeta, type Workspace } from '../schema/workspace';
import { addPage, Section, type App } from './app';

// MaxListed is the most folders, and resources, of a folder which are
// listed, as they're sorted, and paged, in the browser, as a workspace's
// pages are by cursor
const MaxListed = 5000;

// workspacesModule adds the workspaces' page to the app, and each workspace
// to the sidebar, as they're added, and removed
export function workspacesModule(app: App): void {
    const page = addPage<WorkspacePage>(app, 'llm-workspace-page', 'workspace-page', '');
    let workspaces: Workspace[] = [];

    // The folder which is shown, which is listed whole, and then sorted,
    // folders first, and paged
    let name = '';
    let path: string[] = [];
    const controller = new ListController<ResourceMeta>({
        name: 'Files',
        list: async (offset, limit, sort) => {
            if (!name) {
                return { count: 0, body: [] };
            }
            const { items, more } = await listFolder(name, path.join('/'), MaxListed);
            if (more) {
                app.notifications.push({ kind: 'warning', title: `Only the first ${MaxListed} are listed`, subtitle: 'The folder has more' });
            }
            return pageOf(items, offset, limit, sort);
        },
        same: (a, b) => a.uri === b.uri,
        notifications: app.notifications,
        changes: app.changes,
        filter: { schema: 'workspace', table: 'resource' },
        sort: { key: 'name', direction: 'ascending' },
        pageSize: 100,
    });
    page.controller = controller;

    // A folder which is opened is shown, and a resource's content is opened
    // in another tab
    controller.subscribe((state) => {
        const item = state.selected;
        if (!item) {
            return;
        }
        controller.close();
        if (isFolder(item)) {
            location.hash = routeHref('workspace', name, ...path, item.name);
        } else {
            void contentURL(name, item.uri).then((url) => window.open(url, '_blank', 'noopener'));
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
    const show = () => {
        page.workspace = workspaces.find((workspace) => workspace.name === name);
        page.missing = !!name && !page.workspace;

        // Its page is active in the sidebar, once it's there, as the
        // workspaces can be loaded after it's routed to
        if (page.workspace && !page.hidden) {
            app.nav.setActive(`workspace/${name}`);
        }
    };
    app.changes.on({ schema: 'workspace', table: 'workspace' }, () => void refresh());

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
            void controller.reset();
        }
    });
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
