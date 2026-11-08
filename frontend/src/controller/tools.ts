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
import { createConnector, deleteConnector, getConnector, listConnectors, updateConnector } from '../api/connector';
import type { Connector, ConnectorMeta } from '../schema/connector';
import { ConnectorForm } from '../component/connector-form';
import type { ConnectorAlerts } from '../component/connector-alerts';
import { routeHref } from '../router';

export interface ToolsViews {
    page: ToolsPage | null;
    namespaces: CountFilter | null;
    table: ToolTable | null;
    view: ToolView | null;
    agent: AgentEditor | null;
    connector: ConnectorForm | null;
    alerts: ConnectorAlerts | null;
}

// connectWait is how long a connector which is added is waited for, as the
// server connects to it, and connectPoll how often it's asked whether it
// has
const connectWait = 45_000;
const connectPoll = 500;

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

    // The connectors, whose namespaces are those whose type is mcp, so the
    // one of the namespace which is chosen is edited, and deleted
    private connectors: Connector[] = [];

    // How many times the agent's panel has been opened, or closed, so an
    // agent which is loaded once it's been closed, or something else was
    // opened, isn't shown
    private agentGeneration = 0;

    constructor(views: ToolsViews, notifications: NotificationController) {
        super('Tools', notifications);
        this.views = views;
        views.table?.addEventListener(ToolTable.eventPage, (e) => this.setPage((e as CustomEvent<number>).detail));
        views.table?.addEventListener(ToolTable.eventPageSize, (e) => this.setPageSize((e as CustomEvent<number>).detail));
        views.table?.addEventListener(ToolTable.eventSelect, (e) => this.select((e as CustomEvent<Tool>).detail));
        views.view?.addEventListener(Panel.eventClose, () => this.close());

        // An agent is edited, from a new definition, or as it is, and saved,
        // or deleted
        views.page?.addEventListener(ToolsPage.eventNewAgent, () => {
            this.agentGeneration++;
            this.showAgent({ mode: 'new', yaml: agentTemplate });
        });
        views.agent?.addEventListener(AgentEditor.eventSave, (e) => void this.saveAgent((e as CustomEvent<string>).detail));
        views.agent?.addEventListener(AgentEditor.eventDelete, (e) => void this.deleteAgent((e as CustomEvent<string>).detail));
        views.agent?.addEventListener(Panel.eventClose, () => this.hideAgent());

        // An MCP connector is added, and connected, whose tools are added
        views.page?.addEventListener(ToolsPage.eventAddConnector, () => this.showConnector());
        views.connector?.addEventListener(ConnectorForm.eventConnect, (e) => void this.addConnector((e as CustomEvent<ConnectorMeta>).detail));
        views.connector?.addEventListener(ConnectorForm.eventSave, (e) => void this.saveConnector((e as CustomEvent<ConnectorMeta>).detail));
        views.connector?.addEventListener(Panel.eventClose, () => this.hideConnector());
        views.page?.addEventListener(ToolsPage.eventEditConnector, (e) => this.editConnector((e as CustomEvent<string>).detail));
        views.page?.addEventListener(ToolsPage.eventDeleteConnector, (e) => void this.deleteConnector((e as CustomEvent<string>).detail));
    }

    // editConnector opens the panel, to edit the connector, as it is
    private editConnector(name: string) {
        const connector = this.connectors.find((connector) => connector.name === name);
        const form = this.views.connector;
        if (connector && form) {
            form.connecting = false;
            form.error = undefined;
            form.item = { mode: 'edit', connector };
        }
    }

    // saveConnector updates the connector which is edited, and shows the
    // tools of its namespace, as it is now, or says why it couldn't be, in
    // the panel, when it's still editing it
    private async saveConnector(meta: ConnectorMeta) {
        const form = this.views.connector;
        const item = form?.item;
        if (!form || item?.mode !== 'edit') {
            return;
        }
        form.connecting = true;
        try {
            const connector = await updateConnector(item.connector.name, meta);
            this.notifications.push({ kind: 'success', title: `${connector.name} was saved` });
            void this.refresh();
            if (form.item === item) {
                this.hideConnector();
                if (connector.namespace && connector.namespace !== this.namespace) {
                    location.hash = routeHref('tools', connector.namespace);
                }
            }
        } catch (error) {
            if (form.item === item) {
                form.error = error instanceof Error ? error.message : String(error);
            }
        } finally {
            if (form.item === item) {
                form.connecting = false;
            }
        }
    }

    // deleteConnector deletes the connector, once it's confirmed, so its
    // tools are removed, and shows every tool
    private async deleteConnector(name: string) {
        if (!confirm(`Delete the connector ${name}? Its tools can't be used afterwards.`)) {
            return;
        }
        try {
            await deleteConnector(name);
            this.notifications.push({ kind: 'success', title: `${name} was deleted` });
            location.hash = routeHref('tools');
            void this.refresh();
        } catch (error) {
            this.notifications.error(`${name} couldn't be deleted`, error);
        }
    }

    // showConnector opens the panel which adds an MCP connector
    private showConnector() {
        const form = this.views.connector;
        if (form) {
            form.connecting = false;
            form.error = undefined;
            form.item = { mode: 'add' };
        }
    }

    // hideConnector closes the panel which adds an MCP connector
    private hideConnector() {
        if (this.views.connector) {
            this.views.connector.item = undefined;
        }
    }

    // addConnector adds the connector, with a name from its URL's host, or another
    // which isn't taken, and waits for the server to connect to it, when it
    // shows its tools, or says why it couldn't, and deletes it, so a URL which
    // isn't an MCP server's isn't kept, in the panel, when it's still open
    private async addConnector(meta: ConnectorMeta) {
        const form = this.views.connector;
        const item = form?.item;
        if (!form || !item) {
            return;
        }
        form.connecting = true;
        form.error = undefined;
        const open = () => form.item === item;
        let connector: Connector | undefined;
        let why: string;
        try {
            connector = await createConnector(meta);
            const status = await this.waitConnected(connector.name, open);
            if (!open()) {
                // The panel was closed, so the server keeps trying
                return;
            } else if (status.connected) {
                this.notifications.push({ kind: 'success', title: `${status.server?.name ?? status.name} is connected` });
                this.hideConnector();
                void this.refresh();
                if (status.namespace) {
                    location.hash = routeHref('tools', status.namespace);
                }
                return;
            }
            why = status.error ?? "It didn't connect in time";
        } catch (error) {
            // A request which was given up on, as it didn't return in time,
            // didn't connect in time
            why = error instanceof DOMException && error.name === 'TimeoutError'
                ? "It didn't connect in time"
                : error instanceof Error ? error.message : String(error);
        }

        // It couldn't be connected, so it's deleted, while the panel is open,
        // so a URL which isn't an MCP server's isn't kept, and says when it
        // couldn't be, so it's known it may still be stored
        if (!open()) {
            return;
        }
        if (connector) {
            try {
                await deleteConnector(connector.name);
            } catch (error) {
                why += `. The connector ${connector.name} couldn't be deleted, so it may still be stored: ${error instanceof Error ? error.message : String(error)}`;
            }
        }
        if (open()) {
            form.error = why;
            form.connecting = false;
        }
    }

    // waitConnected returns the connector, once the server has connected to
    // it, or couldn't, or it's waited for connectWait, which each request for
    // it is within, too, so one which doesn't return doesn't keep it waiting,
    // or as it was, once it's no longer waited for
    private async waitConnected(name: string, waiting: () => boolean): Promise<Connector> {
        const deadline = Date.now() + connectWait;
        const get = () => getConnector(name, AbortSignal.timeout(Math.max(deadline - Date.now(), 1)));
        let status = await get();
        while (!status.connected && !status.error && Date.now() < deadline && waiting()) {
            await new Promise((resolve) => setTimeout(resolve, connectPoll));
            if (!waiting() || Date.now() >= deadline) {
                break;
            }
            status = await get();
        }
        return status;
    }

    // close closes the tool which is shown, the agent, and the connector's
    // panel, such as when another page is shown, so it doesn't stay open
    // over it, and a connection which is waited for is no longer
    close() {
        super.close();
        this.hideAgent();
        this.hideConnector();
    }

    // select opens the tool, or, for an agent, its definition, to be edited
    private select(tool: Tool) {
        this.agentGeneration++;
        if (toolNamespace(tool) === agentNamespace) {
            void this.openAgent(tool.name);
        } else {
            this.open(tool);
        }
    }

    // openAgent opens the definition of the agent with the name, as YAML, and
    // whether it's built in, unless the panel was closed, or something else
    // was opened, while it was loaded
    private async openAgent(name: string) {
        const generation = this.agentGeneration;
        try {
            const [agent, yaml] = await Promise.all([getAgent(name), getAgentYAML(name)]);
            if (generation === this.agentGeneration) {
                this.showAgent({ mode: 'edit', name: agent.name, yaml, builtin: agent.builtin === true });
            }
        } catch (error) {
            if (generation === this.agentGeneration) {
                this.notifications.error(`The agent ${name} couldn't be loaded`, error);
            }
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
        this.agentGeneration++;
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
    // again, and closes the panel, when it's still showing the agent
    private async deleteAgent(name: string) {
        const panel = this.views.agent;
        const item = panel?.item;
        if (!panel || !item || !confirm(`Delete the agent ${name}?`)) {
            return;
        }
        panel.busy = true;
        try {
            await deleteAgent(name);
            this.notifications.push({ kind: 'success', title: `${name} was deleted` });
            if (panel.item === item) {
                this.hideAgent();
            }
            void this.refresh();
        } catch (error) {
            this.notifications.error(`${name} couldn't be deleted`, error);
        } finally {
            if (panel.item === item) {
                panel.busy = false;
            }
        }
    }

    protected async list(offset: number, limit: number): Promise<Page<Tool>> {
        const [list, namespaces, connectors] = await Promise.all([
            listTools(),
            listNamespaces().catch(() => ({ count: 0, body: [] })),
            listConnectors().catch(() => ({ count: 0, body: [] })),
        ]);
        const tools = list.body ?? [];
        this.namespaces = namespaces.body ?? [];
        this.connectors = connectors.body ?? [];
        this.listeners.forEach((fn) => fn(tools, this.namespaces ?? []));
        const shown = this.namespace ? tools.filter((tool) => toolNamespace(tool) === this.namespace) : tools;
        return { count: shown.length, body: shown.slice(offset, offset + limit) };
    }

    // connectorOf returns the connector of the namespace, which it has
    // whether it's connected, or not
    private connectorOf(namespace: string): Connector | undefined {
        return this.connectors.find((connector) => connector.namespace === namespace);
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
        const { page, alerts, namespaces, table, view } = this.views;
        if (alerts) {
            alerts.connectors = this.connectors;
        }
        if (page) {
            page.namespace = this.namespace;
            page.connector = this.namespace ? this.connectorOf(this.namespace)?.name : undefined;
        }
        if (namespaces) {
            namespaces.tiles = this.namespaces && [
                { label: 'All', count: this.namespaces.reduce((sum, namespace) => sum + namespace.tools, 0) },
                // A connector's namespace has its type as a badge, as a
                // provider has, but built in ones, and the agents', don't,
                // and why it isn't connected, when it isn't, as a provider
                // which can't be reached has
                ...this.namespaces.map((namespace) => {
                    const connector = namespace.type === 'mcp' ? this.connectorOf(namespace.name) : undefined;
                    return {
                        value: namespace.name,
                        label: namespace.name,
                        badge: namespace.type === 'mcp' ? namespace.type : undefined,
                        count: namespace.tools,
                        title: namespace.description,
                        error: connector && !connector.connected ? connector.error : undefined,
                    };
                }),
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
