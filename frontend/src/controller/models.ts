// The models controller, which shows the providers, with how many models
// each has, in the provider filter, and a page of the models, of the provider
// which is chosen, or every provider, in the model table, and the model
// which is clicked in the model view, with ListController, and loads them
// again when the providers change

import { getModel, listModels } from '../api/model';
import { createProvider, deleteProvider, listProviders, updateProvider } from '../api/provider';
import type { ProviderCount, ProviderMeta } from '../schema/provider';
import { ProviderForm } from '../component/provider-form';
import { ModelsPage } from '../page/models-page';
import { routeHref } from '../router';
import type { CountFilter } from '../component/count-filter';
import type { ProviderAlerts } from '../component/provider-alerts';
import { ListController, type Page } from './list';
import type { Model } from '../schema/model';
import type { NotificationController } from './notifications';
import { ModelTable } from '../component/model-table';
import { Panel } from '../component/panel';
import type { ModelView } from '../component/model-view';

export interface ModelsViews {
    page: ModelsPage | null;
    form: ProviderForm | null;
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

        // A provider is added, and the chosen one edited, and deleted, with
        // the form
        views.page?.addEventListener(ModelsPage.eventAdd, (e) => this.showForm({ mode: 'add', preset: (e as CustomEvent<string>).detail }));
        views.page?.addEventListener(ModelsPage.eventEdit, (e) => this.edit((e as CustomEvent<string>).detail));
        views.page?.addEventListener(ModelsPage.eventDelete, (e) => void this.delete((e as CustomEvent<string>).detail));
        views.form?.addEventListener(ProviderForm.eventCreate, (e) => void this.create((e as CustomEvent<ProviderMeta>).detail));
        views.form?.addEventListener(ProviderForm.eventSave, (e) => void this.save((e as CustomEvent<ProviderMeta>).detail));
        views.form?.addEventListener(Panel.eventClose, () => this.hideForm());
    }

    // close closes the model which is shown, and the form
    close() {
        super.close();
        this.hideForm();
    }

    // showForm opens the form, to add a provider, or edit one
    private showForm(item: NonNullable<ProviderForm['item']>) {
        const { form } = this.views;
        if (form) {
            form.saving = false;
            form.error = undefined;
            form.item = item;
        }
    }

    // hideForm closes the form
    private hideForm() {
        if (this.views.form) {
            this.views.form.item = undefined;
        }
    }

    // edit opens the form, to edit the provider, as it is
    private edit(name: string) {
        const provider = this.providers?.find((provider) => provider.name === name);
        if (provider) {
            this.showForm({ mode: 'edit', provider });
        }
    }

    // create adds the provider, which is checked to be reachable, and shows
    // its models, or says why it couldn't be added, in the form
    private async create(meta: ProviderMeta) {
        const form = this.views.form;
        if (!form) {
            return;
        }
        form.saving = true;
        try {
            const provider = await createProvider(meta);
            this.hideForm();
            this.notifications.push({ kind: 'success', title: `${provider.name} was added` });
            location.hash = routeHref('models', provider.name);
        } catch (error) {
            form.error = error instanceof Error ? error.message : String(error);
        } finally {
            form.saving = false;
        }
    }

    // save updates the provider which is edited, or says why it couldn't be,
    // in the form
    private async save(meta: ProviderMeta) {
        const form = this.views.form;
        const item = form?.item;
        if (!form || item?.mode !== 'edit') {
            return;
        }
        form.saving = true;
        try {
            const provider = await updateProvider(item.provider.name, meta);
            this.hideForm();
            this.notifications.push({ kind: 'success', title: `${provider.name} was saved` });
            void this.refresh();
        } catch (error) {
            form.error = error instanceof Error ? error.message : String(error);
        } finally {
            form.saving = false;
        }
    }

    // delete deletes the provider, once it's confirmed, and shows every
    // provider's models
    private async delete(name: string) {
        if (!confirm(`Delete the provider ${name}? Its models can't be used afterwards.`)) {
            return;
        }
        try {
            await deleteProvider(name);
            this.notifications.push({ kind: 'success', title: `${name} was deleted` });
            location.hash = routeHref('models');
        } catch (error) {
            this.notifications.error(`${name} couldn't be deleted`, error);
        }
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
        const { page, alerts, providers, table, view } = this.views;
        if (page) {
            page.provider = this.provider;
        }
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
