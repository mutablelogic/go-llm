// The resources controller, which shows the resources of a chat in the
// resource panel, uploads files to it, deletes them, and opens their
// content, and keeps the resources which are attached to the next message in
// each chat, until it's sent. It loads the resources again when they change,
// such as when a tool returns one.

import { contentURL, createResource, deleteResource, listResources } from '../api/resource';
import type { Resource } from '../schema/resource';
import type { NotificationController } from './notifications';
import { ResourcePanel } from '../component/resource-panel';
import { Panel } from '../component/panel';

export class ResourcesController {
    // The most resources which are listed
    static readonly limit = 100;

    private readonly panel: ResourcePanel | null;
    private readonly notifications: NotificationController;

    // The chat whose resources are shown, and the resources which are
    // attached to the next message, by the chat they're attached in
    private session?: string;
    private readonly attached = new Map<string, Resource[]>();

    // How many files are being uploaded, by the chat they're uploaded to,
    // whose message isn't sent until they are
    private readonly uploads = new Map<string, number>();

    // The load of the resources which is in progress, which is aborted when
    // another starts, so an older one doesn't replace a newer one
    private loading?: AbortController;

    // The functions which are called when the attachments change
    private readonly listeners: (() => void)[] = [];

    constructor(panel: ResourcePanel | null, notifications: NotificationController) {
        this.panel = panel;
        this.notifications = notifications;
        panel?.addEventListener(Panel.eventClose, () => this.close());
        panel?.addEventListener(ResourcePanel.eventAttach, (e) => {
            if (this.session) {
                this.toggle(this.session, (e as CustomEvent<Resource>).detail);
            }
        });
        panel?.addEventListener(ResourcePanel.eventUpload, (e) => {
            if (this.session) {
                void this.upload(this.session, (e as CustomEvent<File[]>).detail);
            }
        });
        panel?.addEventListener(ResourcePanel.eventOpen, (e) => void this.openContent((e as CustomEvent<Resource>).detail));
        panel?.addEventListener(ResourcePanel.eventDelete, (e) => void this.delete((e as CustomEvent<Resource>).detail));
    }

    // onChange calls the function when the attachments, or the uploads,
    // change
    onChange(fn: () => void): this {
        this.listeners.push(fn);
        return this;
    }

    // attachments returns the resources which are attached to the next
    // message in the chat
    attachments(session: string): Resource[] {
        return this.attached.get(session) ?? [];
    }

    // take returns the resources which are attached to the next message in
    // the chat, which are no longer attached, as it's sent
    take(session: string): Resource[] {
        const resources = this.attachments(session);
        this.set(session, []);
        return resources;
    }

    // attach attaches the resources to the next message in the chat, unless
    // they are already, such as when a message couldn't be sent
    attach(session: string, resources: Resource[]) {
        const attached = this.attachments(session);
        this.set(session, [...attached, ...resources.filter((resource) => !attached.some((other) => other.hash === resource.hash))]);
    }

    // detach removes the resource from the next message in the chat
    detach(session: string, resource: Resource) {
        this.set(session, this.attachments(session).filter((other) => other.hash !== resource.hash));
    }

    // open shows the chat's resources in the panel, and loads them
    async open(session: string) {
        this.session = session;
        if (this.panel) {
            this.panel.resources = undefined;
        }
        this.render();
        await this.load();
    }

    // close closes the panel
    close() {
        this.session = undefined;
        this.loading?.abort();
        this.render();
    }

    // uploading returns how many files are being uploaded to the chat
    uploading(session: string): number {
        return this.uploads.get(session) ?? 0;
    }

    // refresh loads the resources again, when the panel is open
    async refresh() {
        if (this.session) {
            await this.load();
        }
    }

    // upload uploads the files to the chat, and attaches them to its next
    // message, as each is uploaded. A file whose content the chat has
    // already is that resource, rather than another.
    async upload(session: string, files: File[]) {
        this.count(session, files.length);
        await Promise.all(files.map(async (file) => {
            try {
                this.attach(session, [await createResource(session, file)]);
            } catch (error) {
                this.notifications.error(`${file.name || 'The file'} couldn't be uploaded`, error);
            } finally {
                this.count(session, -1);
            }
        }));
        await this.refresh();
    }

    // count changes how many files are being uploaded to the chat
    private count(session: string, n: number) {
        const uploading = this.uploading(session) + n;
        if (uploading > 0) {
            this.uploads.set(session, uploading);
        } else {
            this.uploads.delete(session);
        }
        this.changed();
    }

    // load loads the resources of the chat which is shown, aborting the load
    // which is in progress, so an older one doesn't replace it
    private async load() {
        const session = this.session;
        if (!session || !this.panel) {
            return;
        }
        this.loading?.abort();
        const loading = new AbortController();
        this.loading = loading;
        try {
            const list = await listResources(session, 0, ResourcesController.limit, loading.signal);
            if (!loading.signal.aborted && this.session === session) {
                this.panel.resources = list.body ?? [];
                this.panel.count = list.count;
            }
        } catch (error) {
            if (loading.signal.aborted) {
                return;
            }
            if (this.session === session) {
                this.panel.resources = [];
                this.panel.count = 0;
            }
            this.notifications.error("The chat's files couldn't be loaded", error);
        } finally {
            if (this.loading === loading) {
                this.loading = undefined;
            }
        }
    }

    // toggle attaches the resource to the next message in the chat, or
    // removes it, when it's attached
    private toggle(session: string, resource: Resource) {
        if (this.attachments(session).some((other) => other.hash === resource.hash)) {
            this.detach(session, resource);
        } else {
            this.attach(session, [resource]);
        }
    }

    // openContent opens the resource's content in a new tab
    private async openContent(resource: Resource) {
        const url = await contentURL(resource.hash, resource.session ?? this.session);
        window.open(url, '_blank', 'noopener');
    }

    // delete deletes the resource from the chat, once it's confirmed, and
    // removes it from the next message
    private async delete(resource: Resource) {
        const session = this.session;
        if (!session || !confirm(`Delete ${resource.name || 'this file'} from the chat?`)) {
            return;
        }
        try {
            await deleteResource(session, resource.hash);
            this.detach(session, resource);
            await this.load();
        } catch (error) {
            this.notifications.error("The file couldn't be deleted", error);
        }
    }

    private set(session: string, resources: Resource[]) {
        if (resources.length > 0) {
            this.attached.set(session, resources);
        } else {
            this.attached.delete(session);
        }
        this.changed();
    }

    // changed shows the attachments, and the uploads, as they are, and calls
    // the functions which are called when they change
    private changed() {
        this.render();
        for (const fn of this.listeners) {
            fn();
        }
    }

    private render() {
        const panel = this.panel;
        if (!panel) {
            return;
        }
        panel.session = this.session;
        panel.attached = this.session ? this.attachments(this.session).map((resource) => resource.hash) : [];
        panel.uploading = this.session ? this.uploading(this.session) : 0;
    }
}
