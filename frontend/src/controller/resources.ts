// The resources controller, which shows the resources of a chat in the
// resource panel, uploads files to it, deletes them, and opens their
// content, and keeps the resources which are attached to the next message in
// each chat, until it's sent. It loads the resources again when they change,
// such as when a tool returns one.

import { contentURL, createResource, deleteResource, listResources } from '../api/resource';
import { resourceHash, resourceName, resourceTitle, type Resource } from '../schema/resource';
import type { NotificationController } from './notifications';
import { ResourcePanel } from '../component/resource-panel';
import { Panel } from '../component/panel';
import { MediaPlayer, type Media } from '../component/media-player';

export class ResourcesController {
    // The most resources which are listed, and which are known by their
    // names, for the attachments of a chat's messages
    static readonly limit = 100;
    static readonly indexLimit = 1000;

    private readonly panel: ResourcePanel | null;
    private readonly notifications: NotificationController;

    // The chat whose resources are shown, and the resources which are
    // attached to the next message, by the chat they're attached in
    private session?: string;
    private readonly attached = new Map<string, Resource[]>();

    // How many files are being uploaded, by the chat they're uploaded to,
    // whose message isn't sent until they are
    private readonly uploads = new Map<string, number>();

    // The chat whose resources are known, by their hashes, so the
    // attachments of its messages are shown with their names, and the load
    // of them which is in progress
    private indexed?: string;
    private readonly known = new Map<string, Resource>();
    private indexing?: AbortController;

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
        panel?.addEventListener(ResourcePanel.eventPlay, (e) => void this.play((e as CustomEvent<Resource>).detail));
        panel?.addEventListener(ResourcePanel.eventDownload, (e) => void this.download((e as CustomEvent<Resource>).detail));
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

    // refresh loads the resources again, when the panel is open, and those
    // of the chat whose resources are known
    async refresh() {
        await Promise.all([this.session ? this.load() : undefined, this.indexed ? this.index(this.indexed) : undefined]);
    }

    // index loads the chat's resources, so the attachments of its messages
    // are shown with their names, which are forgotten for any other chat
    async index(session: string) {
        if (this.indexed !== session) {
            this.indexed = session;
            this.known.clear();
        }
        this.indexing?.abort();
        const indexing = new AbortController();
        this.indexing = indexing;
        try {
            const list = await listResources(session, 0, ResourcesController.indexLimit, indexing.signal);
            if (!indexing.signal.aborted && this.indexed === session) {
                this.remember(list.body ?? []);
            }
        } catch {
            // Attachments are shown by their types, without their names
        }
    }

    // name returns the name of the resource an attachment refers to, by its
    // URL, such as resource:///<hash>, when it's known
    name(url: string): string | undefined {
        const hash = resourceHash(url);
        const resource = hash ? this.known.get(hash) : undefined;
        return resource ? resourceTitle(resource) : undefined;
    }

    // type returns the type of the resource an attachment refers to, by its
    // URL, such as resource:///<hash>, when it's known
    type(url: string): string | undefined {
        const hash = resourceHash(url);
        return hash ? this.known.get(hash)?.type : undefined;
    }

    // remember keeps the resources of the chat whose resources are known
    private remember(resources: Resource[]) {
        for (const resource of resources) {
            this.known.set(resource.hash, resource);
        }
        this.changed();
    }

    // upload uploads the files to the chat, and attaches them to its next
    // message, as each is uploaded. A file whose content the chat has
    // already is that resource, rather than another.
    async upload(session: string, files: File[]) {
        this.count(session, files.length);
        await Promise.all(files.map(async (file) => {
            try {
                const resource = await createResource(session, file);
                if (this.indexed === session) {
                    this.known.set(resource.hash, resource);
                }
                this.attach(session, [resource]);
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

    // play plays the resource, which is audio, or video, in the player
    private async play(resource: Resource) {
        const url = await contentURL(resource.hash, resource.session ?? this.session);
        const media: Media = { url, name: resourceTitle(resource), type: resource.type };
        document.dispatchEvent(new CustomEvent(MediaPlayer.eventPlay, { detail: media }));
    }

    // download saves the resource's content, with its name, rather than
    // opening it in a tab
    private async download(resource: Resource) {
        const link = document.createElement('a');
        link.href = await contentURL(resource.hash, resource.session ?? this.session, true);
        link.download = resource.name ?? '';
        link.rel = 'noopener';
        document.body.append(link);
        link.click();
        link.remove();
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
            document.dispatchEvent(new CustomEvent(MediaPlayer.eventDeleted, { detail: { hash: resource.hash, session } }));
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
