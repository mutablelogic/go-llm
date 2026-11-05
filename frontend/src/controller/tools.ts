// The tools controller, which shows the namespaces, with how many tools
// each has, in the namespace filter, and a page of the tools, and agents, of
// the namespace which is chosen, or every namespace, in the tool table, and
// the tool which is clicked in the tool view, with ListController, and loads
// them again when they change. The API returns every tool at once, so
// they're filtered, and paged, here, and every tool is given to the
// listeners, such as the new chat's tool picker.

import { getTool, listNamespaces, listTools } from '../api/tool';
import { ListController, type Page } from './list';
import { toolNamespace, type Namespace, type Tool } from '../schema/tool';
import type { NotificationController } from './notifications';
import { ToolTable } from '../component/tool-table';
import { Panel } from '../component/panel';
import type { ToolView } from '../component/tool-view';
import type { CountFilter } from '../component/count-filter';

export interface ToolsViews {
    namespaces: CountFilter | null;
    table: ToolTable | null;
    view: ToolView | null;
}

export class ToolsController extends ListController<Tool> {
    private readonly views: ToolsViews;

    // The functions called with every tool, each time they're loaded
    private readonly listeners: ((tools: Tool[], namespaces: Namespace[]) => void)[] = [];

    // The namespaces, which are undefined while they're first loading, and
    // the namespace whose tools are shown, or none, for every tool
    private namespaces?: Namespace[];
    private namespace?: string;

    constructor(views: ToolsViews, notifications: NotificationController) {
        super('Tools', notifications);
        this.views = views;
        views.table?.addEventListener(ToolTable.eventPage, (e) => this.setPage((e as CustomEvent<number>).detail));
        views.table?.addEventListener(ToolTable.eventPageSize, (e) => this.setPageSize((e as CustomEvent<number>).detail));
        views.table?.addEventListener(ToolTable.eventSelect, (e) => this.open((e as CustomEvent<Tool>).detail));
        views.view?.addEventListener(Panel.eventClose, () => this.close());
    }

    protected async list(offset: number, limit: number): Promise<Page<Tool>> {
        const [list, namespaces] = await Promise.all([listTools(), listNamespaces().catch(() => ({ count: 0, body: [] }))]);
        const tools = list.body ?? [];
        this.namespaces = namespaces.body ?? [];
        this.listeners.forEach((fn) => fn(tools, this.namespaces ?? []));
        const shown = this.namespace ? tools.filter((tool) => toolNamespace(tool) === this.namespace) : tools;
        return { count: shown.length, body: shown.slice(offset, offset + limit) };
    }

    // setNamespace shows the tools of the namespace, or every tool, from the
    // first page
    setNamespace(namespace?: string) {
        if (namespace === this.namespace) {
            return;
        }
        this.namespace = namespace;
        this.page = 1;
        void this.load();
    }

    // onChange calls the function with every tool, and their namespaces,
    // each time they're loaded, such as when they change
    onChange(fn: (tools: Tool[], namespaces: Namespace[]) => void): this {
        this.listeners.push(fn);
        return this;
    }

    protected get(tool: Tool): Promise<Tool> {
        return getTool(tool.canonical);
    }

    protected same(a: Tool, b: Tool): boolean {
        return a.canonical === b.canonical;
    }

    protected render() {
        const { namespaces, table, view } = this.views;
        if (namespaces) {
            namespaces.tiles = this.namespaces && [
                { label: 'All', count: this.namespaces.reduce((sum, namespace) => sum + namespace.tools, 0) },
                ...this.namespaces.map((namespace) => ({ value: namespace.name, label: namespace.name, count: namespace.tools, title: namespace.description })),
            ];
            namespaces.selected = this.namespace;
        }
        if (table) {
            table.tools = this.items;
            table.page = this.page;
            table.pageSize = this.pageSize;
            table.total = this.total;
            table.selected = this.selected?.canonical;
            table.unavailable = this.unavailable;
        }
        if (view) {
            view.tool = this.selected;
        }
    }
}
