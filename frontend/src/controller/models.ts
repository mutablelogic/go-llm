// The models controller, which shows the providers, with how many models
// each has, in the provider filter, and a page of the models, of the provider
// which is chosen, or every provider, in the model table, and the model
// which is clicked in the model view, with ListController, and loads them
// again when the providers change

import { deleteModel, getModel, listModels, loadModel } from '../api/model';
import { providerIcon } from '../schema/preset';
import { ModelLoad } from '../component/model-load';
import { ModelView } from '../component/model-view';
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

export interface ModelsViews {
    page: ModelsPage | null;
    form: ProviderForm | null;
    load: ModelLoad | null;
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
        views.view?.addEventListener(ModelView.eventDelete, (e) => void this.deleteModel((e as CustomEvent<string>).detail));

        // A provider is added, and the chosen one edited, and deleted, with
        // the form
        views.page?.addEventListener(ModelsPage.eventAdd, (e) => this.showForm({ mode: 'add', preset: (e as CustomEvent<string>).detail }));
        views.page?.addEventListener(ModelsPage.eventEdit, (e) => this.edit((e as CustomEvent<string>).detail));
        views.page?.addEventListener(ModelsPage.eventDelete, (e) => void this.delete((e as CustomEvent<string>).detail));
        views.form?.addEventListener(ProviderForm.eventCreate, (e) => void this.create((e as CustomEvent<ProviderMeta>).detail));
        views.form?.addEventListener(ProviderForm.eventSave, (e) => void this.save((e as CustomEvent<ProviderMeta>).detail));
        views.form?.addEventListener(Panel.eventClose, () => this.hideForm());

        // A model is loaded with the chosen provider, whose download is
        // cancelled when the panel is closed
        views.page?.addEventListener(ModelsPage.eventLoad, (e) => this.showLoad((e as CustomEvent<string>).detail));
        views.load?.addEventListener(ModelLoad.eventLoad, (e) => void this.download((e as CustomEvent<string>).detail));
        views.load?.addEventListener(Panel.eventClose, () => this.hideLoad());
    }

    // The download which is in progress, which is aborted when its panel is
    // closed
    private loading?: AbortController;

    // showLoad opens the panel which loads a model with the provider
    private showLoad(name: string) {
        const { load } = this.views;
        const provider = this.providers?.find((provider) => provider.name === name);
        if (load && provider) {
            load.type = provider.provider;
            load.loading = false;
            load.provider = provider.name;
        }
    }

    // hideLoad closes the panel, cancelling the download, when there is one
    private hideLoad() {
        this.loading?.abort();
        this.loading = undefined;
        if (this.views.load) {
            this.views.load.provider = undefined;
            this.views.load.loading = false;
        }
    }

    // download loads the model with the provider of the panel, which
    // downloads it, showing how it's getting on, and then loads the models
    // again, or says why it couldn't be loaded, in the panel
    private async download(name: string) {
        const panel = this.views.load;
        const provider = panel?.provider;
        if (!panel || !provider) {
            return;
        }
        const request = new AbortController();
        this.loading = request;
        panel.loading = true;
        panel.error = undefined;
        panel.progress = undefined;
        try {
            const model = await loadModel(provider, name, (progress) => {
                panel.progress = progress;
            }, request.signal);
            this.notifications.push({ kind: 'success', title: `${model.canonical ?? name} was loaded` });
            this.hideLoad();
            void this.refresh();
        } catch (error) {
            if (!request.signal.aborted) {
                panel.error = error instanceof Error ? error.message : String(error);
            }
        } finally {
            if (this.loading === request) {
                this.loading = undefined;
                panel.loading = false;
            }
        }
    }

    // disconnect shows the models are unavailable, as the server can't be
    // reached, such as when it's stopping, and cancels the download which
    // is in progress, so the server cancels it too, rather than waiting for
    // it as it stops, and says why, in its panel
    disconnect() {
        super.disconnect();
        const request = this.loading;
        if (request) {
            this.loading = undefined;
            request.abort();
            if (this.views.load) {
                this.views.load.loading = false;
                this.views.load.progress = undefined;
                this.views.load.error = "The download was cancelled, as the server can't be reached, such as when it's stopping";
            }
        }
    }

    // close closes the model which is shown, the form, and the panel which
    // loads a model, cancelling its download
    close() {
        super.close();
        this.hideForm();
        this.hideLoad();
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
    // its models, or says why it couldn't be added, in the form, when it's
    // still showing what it was added from, rather than when it was closed,
    // or opened again, as it was added
    private async create(meta: ProviderMeta) {
        const form = this.views.form;
        const item = form?.item;
        if (!form || !item) {
            return;
        }
        form.saving = true;
        try {
            const provider = await createProvider(meta);
            this.notifications.push({ kind: 'success', title: `${provider.name} was added` });
            if (form.item === item) {
                this.hideForm();
                location.hash = routeHref('models', provider.name);
            }
        } catch (error) {
            if (form.item === item) {
                form.error = error instanceof Error ? error.message : String(error);
            }
        } finally {
            if (form.item === item) {
                form.saving = false;
            }
        }
    }

    // save updates the provider which is edited, or says why it couldn't be,
    // in the form, when it's still editing it, rather than when it was
    // closed, or opened again, as it was saved
    private async save(meta: ProviderMeta) {
        const form = this.views.form;
        const item = form?.item;
        if (!form || item?.mode !== 'edit') {
            return;
        }
        form.saving = true;
        try {
            const provider = await updateProvider(item.provider.name, meta);
            this.notifications.push({ kind: 'success', title: `${provider.name} was saved` });
            void this.refresh();
            if (form.item === item) {
                this.hideForm();
            }
        } catch (error) {
            if (form.item === item) {
                form.error = error instanceof Error ? error.message : String(error);
            }
        } finally {
            if (form.item === item) {
                form.saving = false;
            }
        }
    }

    // deleteModel deletes the model from its provider's storage, once it's
    // confirmed, and closes it, and loads the models again
    private async deleteModel(name: string) {
        if (!confirm(`Delete the model ${name} from its provider's storage? It can be downloaded again.`)) {
            return;
        }
        const view = this.views.view;
        if (view) {
            view.busy = true;
        }
        try {
            await deleteModel(name);
            this.notifications.push({ kind: 'success', title: `${name} was deleted` });
            this.close();
            void this.refresh();
        } catch (error) {
            this.notifications.error(`${name} couldn't be deleted`, error);
        } finally {
            if (view) {
                view.busy = false;
            }
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
            page.loadable = this.providers?.find((provider) => provider.name === this.provider)?.downloader === true;
        }
        if (alerts) {
            alerts.providers = this.providers ?? [];
        }
        if (providers) {
            const total = this.providers?.reduce((sum, provider) => sum + (provider.models ?? 0), 0);
            providers.tiles = this.providers && [
                { label: 'All', count: total },
                ...this.providers.map((provider) => ({ value: provider.name, label: provider.name, icon: providerIcon(provider), badge: provider.provider, count: provider.models, error: provider.error })),
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
            // A model can be deleted when its provider stores models,
            // apart from LM Studio's, whose API doesn't delete them
            const provider = this.providers?.find((provider) => provider.name === this.selected?.provider);
            view.stored = provider?.downloader === true && provider.provider !== 'lmstudio';
        }
    }
}
