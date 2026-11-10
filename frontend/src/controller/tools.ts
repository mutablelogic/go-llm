// The tools controller, which shows the namespaces, with how many tools
// each has, in the namespace filter, and a page of the tools, and agents, of
// the namespace which is chosen, or every namespace, in the tool table, and
// the tool which is clicked in the tool view, with ListController, and loads
// them again when they change. The API returns every tool at once, so
// they're filtered, and paged, here, and every tool is given to the
// listeners, such as the new chat's tool picker.

import { getTool, listNamespaces, listTools } from '../api/tool';
import { namespaceIcon } from '../component/namespace-icon';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import LogoMCP16 from '@carbon/icons/es/logo--model-context-protocol/16.js';
import { ListController, type Page } from './list';
import { toolNamespace, type Namespace, type Tool } from '../schema/tool';
import { cannot } from '../schema/model';
import { listModels } from '../api/model';
import type { NotificationController } from './notifications';
import { ToolTable } from '../component/tool-table';
import { Panel } from '../component/panel';
import type { ToolView } from '../component/tool-view';
import type { CountFilter } from '../component/count-filter';
import { AgentEditor, agentTemplate, type AgentEditorItem, type AgentModel } from '../component/agent-editor';
import { ToolsPage } from '../page/tools-page';
import { deleteAgent, getAgent, getAgentYAML, saveAgentYAML, updateAgentModel } from '../api/agent';
import { authorizeConnector, createConnector, deleteConnector, getConnector, listConnectors, listTemplates, updateConnector } from '../api/connector';
import { needsSignIn, type Connector, type ConnectorMeta, type ConnectorTemplate } from '../schema/connector';
import { ConnectorForm } from '../component/connector-form';
import { ConnectorAlerts } from '../component/connector-alerts';
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
// server connects to it, and signInWait how long, once it needs to be
// signed in to, as the person signs in, and connectPoll how often it's
// asked whether it has
const connectWait = 45_000;
const signInWait = 10 * 60_000;
const connectPoll = 500;


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

    // The templates of the servers which a connector can be added for,
    // apart from those which are connectors already
    private templates: ConnectorTemplate[] = [];

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
        views.agent?.addEventListener(AgentEditor.eventModel, (e) => void this.saveAgentModel((e as CustomEvent<AgentModel>).detail));
        views.agent?.addEventListener(Panel.eventClose, () => this.hideAgent());

        // An MCP connector is added, and connected, whose tools are added
        views.page?.addEventListener(ToolsPage.eventAddConnector, (e) => this.showConnector((e as CustomEvent<ConnectorTemplate | undefined>).detail));
        views.connector?.addEventListener(ConnectorForm.eventConnect, (e) => void this.addConnector((e as CustomEvent<ConnectorMeta>).detail));
        views.connector?.addEventListener(ConnectorForm.eventSave, (e) => void this.saveConnector((e as CustomEvent<ConnectorMeta>).detail));
        views.connector?.addEventListener(Panel.eventClose, () => this.hideConnector());
        views.page?.addEventListener(ToolsPage.eventEditConnector, (e) => this.editConnector((e as CustomEvent<string>).detail));
        views.page?.addEventListener(ToolsPage.eventDeleteConnector, (e) => void this.deleteConnector((e as CustomEvent<string>).detail));

        // A connector's server, which needs to be signed in to, is signed in
        // to, in a new tab, from the panel, once it's added, or its alert
        views.connector?.addEventListener(ConnectorForm.eventSignIn, (e) => this.signIn((e as CustomEvent<string>).detail));
        views.alerts?.addEventListener(ConnectorAlerts.eventSignIn, (e) => this.signIn((e as CustomEvent<string>).detail));
        views.alerts?.addEventListener(ConnectorAlerts.eventCancel, (e) => void this.deleteConnector((e as CustomEvent<string>).detail, true));
    }

    // signIn opens a tab, at once, as it's a click, which a browser blocks
    // otherwise, and then at the URL the person signs in to the connector's
    // server at, which redirects back to the server, which connects it, so
    // its event shows it's connected
    private signIn(name: string) {
        void this.authorize(openTab(), name);
    }

    // authorize sends the tab to the URL the person signs in to the
    // connector's server at, or closes it, and says why, when it can't be
    private async authorize(tab: Window | null, name: string) {
        try {
            const url = await authorizeConnector(name);
            if (tab && !tab.closed) {
                tab.location.href = url;
            } else {
                window.open(url, '_blank', 'noopener');
            }
        } catch (error) {
            tab?.close();
            this.notifications.error(`${name} can't be signed in to`, error);
        }
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
    // the panel, when it's still editing it. One which needs to be signed in
    // to is signed in to, once it's saved, in a tab, which is opened at
    // once, as it's a click, and the panel waits for it to be connected
    private async saveConnector(meta: ConnectorMeta) {
        const form = this.views.connector;
        const item = form?.item;
        if (!form || item?.mode !== 'edit') {
            return;
        }
        const tab = needsSignIn(item.connector) ? openTab() : undefined;
        form.connecting = true;
        try {
            const connector = await updateConnector(item.connector.name, meta);
            if (tab !== undefined && !connector.connected) {
                void this.refresh();
                await this.authorize(tab, connector.name);
                return;
            }
            tab?.close();
            this.notifications.push({ kind: 'success', title: `${connector.name} was saved` });
            void this.refresh();
            if (form.item === item) {
                this.hideConnector();
                if (connector.namespace && connector.namespace !== this.namespace) {
                    location.hash = routeHref('tools', connector.namespace);
                }
            }
        } catch (error) {
            tab?.close();
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
    // tools are removed, and shows every tool, or, when cancelling, one which
    // was never signed in to, which is forgotten
    private async deleteConnector(name: string, cancelling = false) {
        const question = cancelling
            ? `Cancel signing in to ${name}? The connector is removed, and can be added again.`
            : `Delete the connector ${name}? Its tools can't be used afterwards.`;
        if (!confirm(question)) {
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

    // showConnector opens the panel which adds an MCP connector, from the
    // template, or a custom one
    private showConnector(template?: ConnectorTemplate) {
        const form = this.views.connector;
        if (form) {
            form.connecting = false;
            form.error = undefined;
            form.signIn = undefined;
            form.item = { mode: 'add', template };
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
            let status = await this.waitConnected(connector.name, open, connectWait, false);

            // One whose server needs to be signed in to is kept, and the
            // panel says so, with Sign in, and waits for it to be connected,
            // once the person has signed in, as its alert does, when the
            // panel's closed, and isn't deleted, whatever happens as it's
            // waited for
            if (open() && needsSignIn(status)) {
                form.connecting = false;
                form.signIn = status.name;
                void this.refresh();
                try {
                    status = await this.waitConnected(status.name, open, signInWait, true);
                } catch (error) {
                    if (open()) {
                        form.signIn = undefined;
                        form.error = `It couldn't be told whether you signed in, so it's kept, and can be signed in to from its alert: ${error instanceof Error ? error.message : String(error)}`;
                    }
                    return;
                }
                if (!open()) {
                    return;
                } else if (!status.connected) {
                    form.signIn = undefined;
                    form.error = status.error && !needsSignIn(status) ? status.error : "It wasn't signed in to in time, so it's kept, and can be signed in to from its alert";
                    return;
                }
                form.signIn = undefined;
            }
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
    // it, or couldn't, which includes when it needs to be signed in to, or
    // it's waited for the time, which each request for it is within, too, so
    // one which doesn't return doesn't keep it waiting, or as it was, once
    // it's no longer waited for. While it's being signed in to, needing to be
    // isn't why it couldn't connect, as the person signs in
    private async waitConnected(name: string, waiting: () => boolean, wait: number, signingIn: boolean): Promise<Connector> {
        const deadline = Date.now() + wait;
        const get = () => getConnector(name, AbortSignal.timeout(Math.max(deadline - Date.now(), 1)));
        let status = await get();
        const done = (connector: Connector) => connector.connected
            || (signingIn ? !needsSignIn(connector) && !!connector.error : !!connector.error || needsSignIn(connector));
        while (!done(status) && Date.now() < deadline && waiting()) {
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
        if (tool.type === 'agent') {
            // An agent, of the agent namespace, or a built-in one, by its
            // name in its namespace, such as builtin:translate
            void this.openAgent(tool.canonical);
        } else {
            this.open(tool);
        }
    }

    // openAgent opens the definition of the agent with the name, as YAML, and
    // whether it's built in, when its model can be chosen from the models
    // which chat, unless the panel was closed, or something else was opened,
    // while it was loaded
    private async openAgent(name: string) {
        const generation = this.agentGeneration;
        try {
            const [agent, yaml] = await Promise.all([getAgent(name), getAgentYAML(name)]);
            const builtin = agent.builtin === true;
            const models = builtin ? await this.chatModels() : undefined;
            if (generation === this.agentGeneration) {
                this.showAgent({ mode: 'edit', name: agent.name, yaml, builtin, model: agent.model, models });
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

    // chatModels returns the canonical names of the models which chat, which
    // a built-in agent's model is chosen from, or none, when they can't be
    // listed
    private async chatModels(): Promise<string[]> {
        try {
            const list = await listModels();
            return (list.body ?? []).filter((model) => !cannot(model, 'chat')).map((model) => model.canonical);
        } catch {
            return [];
        }
    }

    // saveAgentModel sets the built-in agent's model, or clears it, so it's
    // the default, and closes the panel, when it's still showing the agent,
    // or says why it couldn't, in the panel
    private async saveAgentModel({ name, model }: AgentModel) {
        const panel = this.views.agent;
        const item = panel?.item;
        if (!panel || !item) {
            return;
        }
        panel.busy = true;
        try {
            await updateAgentModel(`builtin:${name}`, model);
            this.notifications.push({ kind: 'success', title: `${name} was saved` });
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
        const [list, namespaces, connectors, templates] = await Promise.all([
            listTools(),
            listNamespaces().catch(() => ({ count: 0, body: [] })),
            listConnectors().catch(() => ({ count: 0, body: [] })),
            listTemplates().catch(() => ({ count: 0, body: [] })),
        ]);
        this.templates = templates.body ?? [];
        const tools = list.body ?? [];
        this.namespaces = namespaces.body ?? [];
        this.connectors = connectors.body ?? [];
        this.listeners.forEach((fn) => fn(tools, this.namespaces ?? []));
        const shown = this.namespace ? tools.filter((tool) => inNamespace(tool, this.namespace!)) : tools;
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
        const { page, alerts, namespaces, table, view, connector: form } = this.views;

        // The connector which is edited, which needed to be signed in to,
        // and now is connected, such as once it's signed in to from its
        // panel, is shown, with its tools
        if (form?.item?.mode === 'edit' && needsSignIn(form.item.connector)) {
            const name = form.item.connector.name;
            const now = this.connectors.find((connector) => connector.name === name);
            if (now?.connected) {
                this.notifications.push({ kind: 'success', title: `${now.server?.name ?? now.name} is connected` });
                this.hideConnector();
                if (now.namespace && now.namespace !== this.namespace) {
                    location.hash = routeHref('tools', now.namespace);
                }
            }
        }
        if (alerts) {
            alerts.connectors = this.connectors;
        }
        if (page) {
            page.namespace = this.namespace;
            page.connector = this.namespace ? this.connectorOf(this.namespace)?.name : undefined;
            page.templates = this.templates;
        }
        if (namespaces) {
            namespaces.tiles = this.namespaces && [
                { label: 'All', count: this.namespaces.reduce((sum, namespace) => sum + namespace.tools, 0) },
                // A connector's namespace has its type as a badge, as a
                // provider has, but built in ones, and the agents', don't,
                // and why it isn't connected, when it isn't, as a provider
                // which can't be reached has
                ...mergeAgents(this.namespaces).map((namespace) => {
                    const connector = namespace.type === 'mcp' ? this.connectorOf(namespace.name) : undefined;
                    return {
                        value: namespace.name,
                        label: namespace.name,
                        badge: namespace.type === 'mcp' ? namespace.type : undefined,
                        // A connector's logo, or MCP's, when it has none
                        icon: namespace.type === 'mcp' ? connector?.icon ?? iconLoader(LogoMCP16) : namespaceIcon(namespace.name),
                        count: namespace.tools,
                        title: namespace.description,
                        error: connector && needsSignIn(connector) ? 'Needs you to sign in' : connector && !connector.connected ? connector.error : undefined,
                        status: connector && needsSignIn(connector) ? 'Unauthorized' : undefined,
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

// openTab opens a blank tab, at once, as a click opens it, which a browser
// blocks once it's waited for a request, which the sign-in page is sent to,
// once its URL is got, and which can't reach back to this page
function openTab(): Window | null {
    const tab = window.open('about:blank', '_blank');
    if (tab) {
        tab.opener = null;
    }
    return tab;
}

// agentNamespaces are the namespaces of the agents, which the page shows as
// one, agent: those which people, and chats, make, and the server's own,
// which are built in
const agentNamespaces = ['agent', 'builtin'];

// mergeAgents returns the namespaces, with the agents' as one, agent, where
// the first of them is, whose tools are both's, so the built-in agents are
// with the rest
function mergeAgents(namespaces: Namespace[]): Namespace[] {
    const agents = namespaces.filter((namespace) => agentNamespaces.includes(namespace.name));
    if (agents.length === 0) {
        return namespaces;
    }
    const own = agents.find((namespace) => namespace.name === 'agent') ?? agents[0];
    const merged: Namespace = { ...own, name: 'agent', type: 'agent', tools: agents.reduce((sum, namespace) => sum + namespace.tools, 0) };
    const result: Namespace[] = [];
    for (const namespace of namespaces) {
        if (!agentNamespaces.includes(namespace.name)) {
            result.push(namespace);
        } else if (!result.includes(merged)) {
            result.push(merged);
        }
    }
    return result;
}

// inNamespace returns whether the tool is in the namespace, as the page
// shows it, where the agent namespace has the built-in agents too
function inNamespace(tool: Tool, namespace: string): boolean {
    const own = toolNamespace(tool);
    return namespace === 'agent' ? agentNamespaces.includes(own) : own === namespace;
}
