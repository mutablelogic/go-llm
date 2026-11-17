// The Tools module: the tools and agents which models call, in a table, with
// the one which is clicked in a panel, agents, which are written in an
// editor, and the connectors to MCP servers, whose tools they are

import { CountFilter } from '../component/count-filter';
import { ToolsController } from '../controller/tools';
import type { ToolsPage } from '../page/tools-page';
import { routeHref } from '../router';
import { addPage, Section, type App } from './app';

// toolsModule adds the Tools page, at #tools, or #tools/<namespace>, to the
// app
export function toolsModule(app: App): ToolsController {
    const page = addPage<ToolsPage>(app, 'llm-tools-page', 'tools-page', `
        <div class="content">
            <llm-connector-alerts></llm-connector-alerts>
            <llm-count-filter label="Filter by namespace"></llm-count-filter>
            <llm-tool-table></llm-tool-table>
        </div>
        <llm-tool-view></llm-tool-view>
        <llm-agent-editor></llm-agent-editor>
        <llm-connector-form></llm-connector-form>
    `);
    const namespaces = page.querySelector<CountFilter>('llm-count-filter');
    const tools = new ToolsController({
        page,
        namespaces,
        table: page.querySelector('llm-tool-table'),
        view: page.querySelector('llm-tool-view'),
        agent: page.querySelector('llm-agent-editor'),
        connector: page.querySelector('llm-connector-form'),
        alerts: page.querySelector('llm-connector-alerts'),
    }, app.notifications);

    // A namespace which is clicked shows only its tools, such as
    // #tools/memory
    namespaces?.addEventListener(CountFilter.eventSelect, (e) => {
        const namespace = (e as CustomEvent<string | undefined>).detail;
        location.hash = namespace ? routeHref('tools', namespace) : routeHref('tools');
    });

    // The tools, and agents, and connectors, as they're connected, and
    // disconnected
    app.changes
        .on([{ schema: 'llm', table: 'tool' }, { schema: 'mcp', table: 'connector' }], () => void tools.refresh())
        .onDisconnect(() => tools.disconnect())
        .onConnect(() => tools.connect());

    app.nav.setModule('tools', Section, [{ id: 'tools', label: 'Tools', href: '#tools' }]);
    app.router.add('tools', page, (route) => {
        tools.setNamespace(route.params[0] || undefined);
        void tools.refresh();
    });
    app.router.onRoute((route) => {
        if (route.page !== 'tools') {
            tools.close();
        }
    });
    void tools.load();
    return tools;
}
