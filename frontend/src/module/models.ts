// The Models module: the providers' models, in a table, with the one which
// is clicked in a panel, and the providers, which are added, and changed,
// in a form

import { CountFilter } from '../component/count-filter';
import { ModelsController } from '../controller/models';
import type { ModelsPage } from '../page/models-page';
import { routeHref } from '../router';
import { addPage, Section, type App } from './app';

// modelsModule adds the Models page, at #models, or #models/<provider>, to
// the app
export function modelsModule(app: App): ModelsController {
    const page = addPage<ModelsPage>(app, 'llm-models-page', 'models-page', `
        <div class="content">
            <llm-provider-alerts></llm-provider-alerts>
            <llm-count-filter label="Filter by provider"></llm-count-filter>
            <llm-model-table></llm-model-table>
        </div>
        <llm-model-view></llm-model-view>
        <llm-provider-form></llm-provider-form>
        <llm-model-load></llm-model-load>
    `);
    const providers = page.querySelector<CountFilter>('llm-count-filter');
    const models = new ModelsController({
        page,
        form: page.querySelector('llm-provider-form'),
        load: page.querySelector('llm-model-load'),
        alerts: page.querySelector('llm-provider-alerts'),
        providers,
        table: page.querySelector('llm-model-table'),
        view: page.querySelector('llm-model-view'),
    }, app.notifications);

    // A provider which is clicked shows only its models, which the location
    // says, such as #models/ollama, so it's linked to, and the back button
    // shows every model again
    providers?.addEventListener(CountFilter.eventSelect, (e) => {
        const provider = (e as CustomEvent<string | undefined>).detail;
        location.hash = provider ? routeHref('models', provider) : routeHref('models');
    });

    // The providers, whose models change
    app.changes
        .on({ schema: 'llm', table: 'provider' }, () => void models.refresh())
        .onDisconnect(() => models.disconnect())
        .onConnect(() => models.connect());

    app.nav.setModule('models', Section, [{ id: 'models', label: 'Models', href: '#models' }]);
    app.router.add('models', page, (route) => {
        models.setProvider(route.params[0] || undefined);
        void models.refresh();
    });
    app.router.onRoute((route) => {
        if (route.page !== 'models') {
            models.close();
        }
    });
    void models.load();
    return models;
}
