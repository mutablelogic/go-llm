// The models controller, which shows the providers, with how many models
// each has, in the provider filter, and a page of the models, of the provider
// which is chosen, or every provider, in the model table, and the model
// which is clicked in the model view, with ListController, and loads them
// again when the providers change

import { getModel, listModels } from '../api/model';
import { listProviders } from '../api/provider';
import type { ProviderCount } from '../schema/provider';
import type { CountFilter } from '../component/count-filter';
import type { ProviderAlerts } from '../component/provider-alerts';
import { ListController, type Page } from './list';
import type { Model } from '../schema/model';
import type { NotificationController } from './notifications';
import { ModelTable } from '../component/model-table';
import { Panel } from '../component/panel';
import type { ModelView } from '../component/model-view';

export interface ModelsViews {
    alerts: ProviderAlerts | null;
    providers: CountFilter | null;
    table: ModelTable | null;
    view: ModelView | null;
}

export class ModelsController extends ListController<Model> {
    private readonly views: ModelsViews;

    // The providers, with how many models each has, which are undefined
    // while they're first loading, and the provider whose models are shown,
    // or none, for every provider's
    private providers?: ProviderCount[];
    private provider?: string;

    constructor(views: ModelsViews, notifications: NotificationController) {
        super('Models', notifications);
        this.views = views;
        views.table?.addEventListener(ModelTable.eventPage, (e) => this.setPage((e as CustomEvent<number>).detail));
        views.table?.addEventListener(ModelTable.eventPageSize, (e) => this.setPageSize((e as CustomEvent<number>).detail));
        views.table?.addEventListener(ModelTable.eventSelect, (e) => this.open((e as CustomEvent<Model>).detail));
        views.view?.addEventListener(Panel.eventClose, () => this.close());
    }

    // setProvider shows the models of the provider, or every provider, from
    // the first page
    setProvider(provider?: string) {
        if (provider === this.provider) {
            return;
        }
        this.provider = provider;
        this.page = 1;
        void this.load();
    }

    // list returns the page of the models of the provider, or every provider,
    // and loads the providers again, with how many models each has
    protected async list(offset: number, limit: number): Promise<Page<Model>> {
        await this.loadProviders();
        if (this.unreachable()) {
            return { count: 0, body: [] };
        }
        const models = (await listModels(this.provider)).body ?? [];
        return { count: models.length, body: models.slice(offset, offset + limit) };
    }

    // announce says which providers can no longer be reached, and which can
    // be again, since they were last loaded, but not when they're first
    // loaded, as the alerts say which can't
    private announce(providers: ProviderCount[]) {
        const before = this.providers;
        if (!before) {
            return;
        }
        for (const provider of providers) {
            const was = before.find((other) => other.name === provider.name);
            if (!was) {
                continue;
            } else if (provider.error && !was.error) {
                this.notifications.push({ kind: 'error', title: `${provider.name} can't be reached`, subtitle: provider.error });
            } else if (!provider.error && was.error) {
                this.notifications.push({ kind: 'success', title: `${provider.name} can be reached again` });
            }
        }
    }

    // unreachable returns the provider which is chosen, when it can't be
    // reached, whose models aren't asked for, as they can't be listed
    private unreachable(): ProviderCount | undefined {
        return this.providers?.find((provider) => provider.name === this.provider && provider.error);
    }

    // loadProviders loads the providers, and counts their models, which
    // aren't counted for a provider which can't be reached
    private async loadProviders() {
        try {
            const providers = (await listProviders()).body ?? [];
            this.announce(providers);
            this.providers = await Promise.all(providers.map(async (provider) => {
                if (provider.error) {
                    return { ...provider };
                }
                try {
                    return { ...provider, models: (await listModels(provider.name)).count };
                } catch {
                    return { ...provider };
                }
            }));
        } catch (error) {
            this.providers ??= [];
            this.notifications.error("Providers couldn't be loaded", error);
        }
        this.render();
    }

    protected get(model: Model): Promise<Model> {
        return getModel(model.canonical);
    }

    protected same(a: Model, b: Model): boolean {
        return a.canonical === b.canonical;
    }

    protected render() {
        const { alerts, providers, table, view } = this.views;
        if (alerts) {
            alerts.providers = this.providers ?? [];
        }
        if (providers) {
            const total = this.providers?.reduce((sum, provider) => sum + (provider.models ?? 0), 0);
            providers.tiles = this.providers && [
                { label: 'All', count: total },
                ...this.providers.map((provider) => ({ value: provider.name, label: provider.name, badge: provider.provider, count: provider.models, error: provider.error })),
            ];
            providers.selected = this.provider;
        }
        if (table) {
            table.models = this.items;
            table.page = this.page;
            table.pageSize = this.pageSize;
            table.total = this.total;
            table.selected = this.selected?.canonical;
            table.unavailable = this.unavailable;

        }
        if (view) {
            view.model = this.selected;
        }
    }
}
