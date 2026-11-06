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
import { AgentEditor, agentTemplate, type AgentEditorItem } from '../component/agent-editor';
import { ToolsPage } from '../page/tools-page';
import { deleteAgent, getAgent, getAgentYAML, saveAgentYAML } from '../api/agent';

export interface ToolsViews {
    page: ToolsPage | null;
    namespaces: CountFilter | null;
    table: ToolTable | null;
    view: ToolView | null;
    agent: AgentEditor | null;
}

// agentNamespace is the namespace of the agents, as tools
const agentNamespace = 'agent';

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
        views.table?.addEventListener(ToolTable.eventSelect, (e) => this.select((e as CustomEvent<Tool>).detail));
        views.view?.addEventListener(Panel.eventClose, () => this.close());

        // An agent is edited, from a new definition, or as it is, and saved,
        // or deleted
        views.page?.addEventListener(ToolsPage.eventNewAgent, () => this.showAgent({ mode: 'new', yaml: agentTemplate }));
        views.agent?.addEventListener(AgentEditor.eventSave, (e) => void this.saveAgent((e as CustomEvent<string>).detail));
        views.agent?.addEventListener(AgentEditor.eventDelete, (e) => void this.deleteAgent((e as CustomEvent<string>).detail));
        views.agent?.addEventListener(Panel.eventClose, () => this.hideAgent());
    }

    // close closes the tool which is shown, and the agent
    close() {
        super.close();
        this.hideAgent();
    }

    // select opens the tool, or, for an agent, its definition, to be edited
    private select(tool: Tool) {
        if (toolNamespace(tool) === agentNamespace) {
            void this.openAgent(tool.name);
        } else {
            this.open(tool);
        }
    }

    // openAgent opens the definition of the agent with the name, as YAML, and
    // whether it's built in
    private async openAgent(name: string) {
        try {
            const [agent, yaml] = await Promise.all([getAgent(name), getAgentYAML(name)]);
            this.showAgent({ mode: 'edit', name: agent.name, yaml, builtin: agent.builtin === true });
        } catch (error) {
            this.notifications.error(`The agent ${name} couldn't be loaded`, error);
        }
    }

    // showAgent opens the agent's panel
    private showAgent(item: AgentEditorItem) {
        const { agent } = this.views;
        if (agent) {
            agent.busy = false;
            agent.error = undefined;
            agent.item = item;
        }
    }

    // hideAgent closes the agent's panel
    private hideAgent() {
        if (this.views.agent) {
            this.views.agent.item = undefined;
        }
    }

    // saveAgent creates, or replaces, the agent with the definition, and
    // loads the tools again, or says why it couldn't, in the panel, when it's
    // still showing what was saved
    private async saveAgent(yaml: string) {
        const panel = this.views.agent;
        const item = panel?.item;
        if (!panel || !item) {
            return;
        }
        panel.busy = true;
        try {
            const agent = await saveAgentYAML(yaml);
            this.notifications.push({ kind: 'success', title: `${agent.name} was saved` });
            void this.refresh();
            if (panel.item === item) {
                this.hideAgent();
            }
        } catch (error) {
            if (panel.item === item) {
                panel.error = error instanceof Error ? error.message : String(error);
            }
        } finally {
            if (panel.item === item) {
                panel.busy = false;
            }
        }
    }

    // deleteAgent deletes the agent, once it's confirmed, and loads the tools
    // again
    private async deleteAgent(name: string) {
        if (!confirm(`Delete the agent ${name}?`)) {
            return;
        }
        const panel = this.views.agent;
        if (panel) {
            panel.busy = true;
        }
        try {
            await deleteAgent(name);
            this.notifications.push({ kind: 'success', title: `${name} was deleted` });
            this.hideAgent();
            void this.refresh();
        } catch (error) {
            this.notifications.error(`${name} couldn't be deleted`, error);
        } finally {
            if (panel) {
                panel.busy = false;
            }
        }
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
        const { page, namespaces, table, view } = this.views;
        if (page) {
            page.namespace = this.namespace;
        }
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
