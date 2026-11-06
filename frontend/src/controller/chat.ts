// The chat controller, which shows the sessions in the session list, and the
// conversation of the one which is open in the chat view, sends questions,
// and shows the replies as they're streamed, says someone is typing, starts
// new chats, and deletes them. It loads them again when they change, such as
// when another window replies in a session, and the names of the person and
// the assistant, from the memory, when they change. The chat which is open
// is kept, so it's opened again when the chat page is shown without one,
// such as when the browser is opened. A question is sent with the resources
// which are attached to it, which the resources controller keeps.

import { chat, createSession, deleteSession, getSession, listMessages, listSessions, setStatus, updateSession } from '../api/session';
import { getDetails } from '../api/memory';
import { listModels } from '../api/model';
import { cannot } from '../schema/model';
import { ApiError } from '../api/client';
import { routeHref } from '../router';
import { isUser, sessionTokens, type ContentBlock, type Message, type Session, type SessionMeta } from '../schema/session';
import type { Details } from '../schema/memory';
import type { NotificationController } from './notifications';
import type { StorageController } from './storage';
import type { ResourcesController } from './resources';
import { resourceName, resourceURI, type Resource } from '../schema/resource';
import { SessionList } from '../component/session-list';
import { ChatView, type Piece } from '../component/chat-view';
import { NewSession } from '../component/new-session';
import type { MessageView } from '../component/message-view';
import { Panel } from '../component/panel';
import { ChatPage } from '../page/chat-page';

export interface ChatViews {
    page: ChatPage | null;
    list: SessionList | null;
    view: ChatView | null;
    create: NewSession | null;
    details: MessageView | null;
}

export class ChatController {
    // The most sessions which are listed
    static readonly limit = 100;

    // How often the server is told someone is still typing, which it
    // forgets after ten seconds
    static readonly typingInterval = 5000;

    private readonly views: ChatViews;
    private readonly notifications: NotificationController;
    private readonly storage: StorageController;
    private readonly resources: ResourcesController;

    // The sessions, which are undefined while they're first loading, the
    // session which is open, its messages, and the memory's details
    private sessions?: Session[];
    private session?: Session;
    private messages?: Message[];
    private details: Details = {};

    // The message whose details are shown
    private selected?: Message;

    // The lines tools logged as replies were streamed, which aren't stored,
    // so they're kept, by the session, and the identifier of the result of
    // the first tool its reply called, or the position of its last message,
    // when it called none, so they're shown where they were, until the page
    // is closed
    private readonly logs = new Map<string, string[]>();

    // The questions which are waiting for their replies, by the sessions
    // they were asked in, so a reply is shown only in its own chat, and
    // when the server was last told someone is typing
    private readonly pending = new Map<string, Pending>();
    private typed = 0;

    // Whether the server can't be reached
    private unavailable = false;

    constructor(views: ChatViews, notifications: NotificationController, storage: StorageController, resources: ResourcesController) {
        this.views = views;
        this.notifications = notifications;
        this.storage = storage;
        this.resources = resources;
        resources.onChange(() => this.render());
        views.view?.addEventListener(ChatView.eventAttach, () => {
            if (this.session) {
                void resources.open(this.session.id);
            }
        });
        views.view?.addEventListener(ChatView.eventDetach, (e) => {
            if (this.session) {
                resources.detach(this.session.id, (e as CustomEvent<Resource>).detail);
            }
        });
        views.view?.addEventListener(ChatView.eventUpload, (e) => {
            if (this.session) {
                void resources.upload(this.session.id, (e as CustomEvent<File[]>).detail);
            }
        });
        views.list?.addEventListener(SessionList.eventSelect, (e) => {
            location.hash = routeHref('chat', (e as CustomEvent<Session>).detail.id);
        });
        views.page?.addEventListener(ChatPage.eventNew, () => void this.newChat());
        views.list?.addEventListener(SessionList.eventDelete, () => void this.delete());
        views.list?.addEventListener(SessionList.eventEdit, () => void this.edit());
        views.create?.addEventListener(NewSession.eventSave, (e) => void this.save((e as CustomEvent<SessionMeta>).detail));
        views.view?.addEventListener(ChatView.eventSend, (e) => void this.send((e as CustomEvent<string>).detail));
        views.view?.addEventListener(ChatView.eventTyping, () => this.typing());
        views.view?.addEventListener(ChatView.eventInfo, (e) => {
            this.selected = (e as CustomEvent<Message>).detail;
            this.render();
        });
        views.details?.addEventListener(Panel.eventClose, () => {
            this.selected = undefined;
            this.render();
        });
        views.create?.addEventListener(NewSession.eventCreate, (e) => void this.create((e as CustomEvent<SessionMeta>).detail));
        views.create?.addEventListener(Panel.eventClose, () => this.hideCreate());
    }

    // open opens the session, or none, and loads its messages
    async open(id?: string) {
        // Without a chat, the one which was last open is opened, in place of
        // the location without one, and a chat which is opened is kept
        const last = this.storage.get('chat');
        if (!id && last) {
            location.replace(routeHref('chat', last));
            return;
        } else if (id) {
            this.storage.set('chat', id);
        }
        if (id === this.session?.id && this.messages) {
            return;
        }
        this.resources.close();
        this.session = this.sessions?.find((session) => session.id === id) ?? (id ? { id, serial: 0 } : undefined);
        this.messages = undefined;
        this.selected = undefined;
        this.render();
        if (id) {
            await this.load(id);
            this.views.view?.focus();
        }
    }

    // refresh loads the sessions again, and the session which is open, and
    // its messages, when it's changed, unless a reply is waited for, when
    // it's loaded once the reply is stored
    async refresh() {
        try {
            this.sessions = (await listSessions(0, ChatController.limit)).body ?? [];
        } catch (error) {
            this.sessions ??= [];
            this.notifications.error("Chats couldn't be loaded", error);
        }
        if (this.session) {
            await this.load(this.session.id);
        }
        this.render();
    }

    // refreshDetails loads the memory's details again, for the names of the
    // person and the assistant, which a server without a memory hasn't
    async refreshDetails() {
        try {
            this.details = await getDetails();
            this.render();
        } catch {
            // The names aren't known
        }
    }

    // disconnect shows the chats as unavailable, as the server can't be
    // reached, and connect shows them as they are, once it can be again
    disconnect() {
        this.unavailable = true;
        this.render();
    }

    connect() {
        this.unavailable = false;
        this.render();
    }

    // load loads the session, and its messages, when it's changed since they
    // were loaded, or they weren't, or closes it, when it was deleted
    private async load(id: string) {
        try {
            const session = await getSession(id);
            if (this.session?.id !== id) {
                return;
            }
            // While a reply in the session is waited for, its messages are
            // loaded once it's stored, and the session's serial is only
            // changed once its messages are loaded, so a load which fails is
            // tried again
            const changed = !this.messages || session.serial !== this.session.serial;
            const pending = this.pending.get(id);
            if (changed && pending && this.messages) {
                pending.stale = true;
                this.session = { ...session, serial: this.session.serial };
            } else if (changed) {
                const messages = (await listMessages(id)).body ?? [];
                if (this.session?.id === id) {
                    this.session = session;
                    this.messages = messages;
                }
            } else {
                this.session = session;
            }
        } catch (error) {
            // A chat which isn't there, or can't be, isn't opened again
            if (error instanceof ApiError && (error.code === 400 || error.code === 404)) {
                this.storage.delete('chat', id);
            }
            if (error instanceof ApiError && error.code === 404) {
                this.notifications.push({ kind: 'info', title: 'The chat was deleted' });
                location.hash = routeHref('chat');
            } else {
                this.notifications.error("The chat couldn't be loaded", error);
            }
        }
        this.render();
    }

    // send sends the question, and shows the reply as it's streamed, in its
    // own chat, and then adds it to the conversation, when its chat is open,
    // or puts the question back, when it couldn't be sent, in its own chat,
    // or says what it was, when another is open
    private async send(text: string) {
        const session = this.session;
        if (!session || !this.messages || this.pending.has(session.id)) {
            return;
        }
        // The attachments come before the text, as the command line's do
        const attachments = this.resources.take(session.id);
        const content: ContentBlock[] = attachments.map((resource) => ({ attachment: { type: resource.type, url: resourceURI(resource) } }));
        if (text) {
            content.push({ text });
        }
        const question = [...attachments.map((resource) => `Attachment: ${resourceName(resource)}`), text].filter(Boolean).join('\n');
        const pending: Pending = { question, streamed: [], earlier: this.messages.length, stale: false };
        this.pending.set(session.id, pending);
        this.render();
        try {
            const response = await chat(session.id, { role: 'user', content }, {
                onText: (piece) => this.add(pending, { what: 'text', text: piece }),
                onThinking: (piece) => this.add(pending, { what: 'thinking', text: piece }),
                onLog: (line) => this.add(pending, { what: 'log', text: line }),
                onProgress: (progress) => {
                    pending.progress = progress.message || progress.status;
                    this.render();
                },
            });
            const added = response.messages ?? [];
            this.keepLogs(session.id, pending, added);
            if (this.session?.id === session.id && this.messages && !pending.stale) {
                this.messages = [...this.messages, ...added];
                this.session = { ...this.session, serial: response.serial };
            } else {
                pending.stale = true;
            }
        } catch (error) {
            if (this.session?.id === session.id) {
                this.notifications.error("The question couldn't be sent", error);
                this.views.view?.restore(text);
                this.resources.attach(session.id, attachments);
            } else {
                this.notifications.push({ kind: 'error', title: "A question in another chat couldn't be sent", subtitle: `${error instanceof Error ? error.message : String(error)}: ${question}` });
                this.resources.attach(session.id, attachments);
            }
            pending.stale = true;
        }
        this.pending.delete(session.id);
        this.typed = 0;
        this.render();
        if (pending.stale && this.session?.id === session.id) {
            this.messages = undefined;
            await this.load(session.id);
        }
        void this.refresh();
    }

    // keepLogs keeps the lines tools logged, as the reply was streamed, by
    // the result of the first tool it called, or else the position of its
    // last message, of the messages which were added after the earlier ones
    private keepLogs(id: string, pending: Pending, added: Message[]) {
        const lines = pending.streamed.filter((piece) => piece.what === 'log').map((piece) => piece.text.trim());
        if (lines.length === 0) {
            return;
        }
        const result = added.flatMap((message) => message.content).find((block) => block.tool_result?.id)?.tool_result?.id;
        const key = logKey(id, result, pending.earlier + added.length - 1);
        this.logs.set(key, [...(this.logs.get(key) ?? []), ...lines]);
    }

    // describe returns what's said of the chat which is open, beneath its
    // heading: its model, when the heading is its title, the tokens of its
    // conversation, and whether someone is typing, or its model is
    // replying, or how to start one, when none is
    private describe(): string {
        const session = this.session;
        if (!session) {
            return 'Choose a chat, or start a new one';
        }
        // The model, when the heading is the title, rather than the model
        const parts: string[] = [];
        if (session.title && session.model) {
            parts.push(session.model);
        }
        const tokens = sessionTokens(session);
        if (tokens) {
            parts.push(`${tokens.toLocaleString()} tokens`);
        }
        if (this.pending.has(session.id) || session.status === 'replying') {
            parts.push('replying…');
        } else if (session.status === 'typing') {
            parts.push('typing…');
        }
        return parts.join(' · ');
    }

    // add adds a piece of the reply, as it's streamed
    private add(pending: Pending, piece: Piece) {
        pending.streamed = [...pending.streamed, piece];
        pending.progress = undefined;
        this.render();
    }

    // sessionID returns the identifier of the chat which is open, or
    // undefined when none is
    sessionID(): string | undefined {
        return this.session?.id;
    }

    // close closes the chat's panels, which are the new, or edited, chat,
    // and a message's details, such as when the chat page isn't shown, so
    // they don't stay open over another page
    close() {
        this.hideCreate();
        this.resources.close();
        if (this.selected) {
            this.selected = undefined;
            this.render();
        }
    }

    // typing tells the server someone is typing in the session, at most
    // every few seconds
    private typing() {
        const now = Date.now();
        if (!this.session || now - this.typed < ChatController.typingInterval) {
            return;
        }
        this.typed = now;
        setStatus(this.session.id, 'typing').catch(() => {
            // The status is only shown, so isn't worth an error
        });
    }

    // newChat shows the panel to start a new chat, with the models which
    // aren't known not to generate text, choosing the model, or else the
    // model of the chat which is open, or the latest, first
    async newChat(model?: string) {
        const create = this.views.create;
        if (!create) {
            return;
        }
        create.session = undefined;
        create.model = model ?? this.session?.model ?? this.sessions?.[0]?.model;
        create.chosen = model;
        create.models = undefined;
        create.visible = true;
        try {
            const models = (await listModels()).body ?? [];
            create.models = models.filter((model) => !cannot(model, 'completion'));
        } catch (error) {
            create.models = [];
            this.notifications.error("Models couldn't be loaded", error);
        }
    }

    // edit shows the panel to edit the chat which is open: its model, system
    // prompt, options and tools
    private async edit() {
        const create = this.views.create;
        if (!create || !this.session) {
            return;
        }
        const session = this.session;
        create.session = session;
        create.model = session.model;
        create.models = undefined;
        create.visible = true;
        try {
            // The chat's model is kept, as it is, when it isn't listed, such
            // as when its provider can't be reached, so a change to the chat
            // doesn't change its model, unless it's chosen
            const models = (await listModels()).body ?? [];
            const chosen = models.filter((model) => !cannot(model, 'completion') || model.canonical === session.model);
            if (session.model && !chosen.some((model) => model.canonical === session.model)) {
                const colon = session.model.indexOf(':');
                chosen.unshift({ name: session.model.slice(colon + 1), provider: session.model.slice(0, colon), canonical: session.model });
            }
            create.models = chosen;
        } catch (error) {
            create.models = [];
            this.notifications.error("Models couldn't be loaded", error);
        }
    }

    // save changes the settings of the chat which is edited, when they've
    // changed, and shows it as it is, when it's still open
    private async save(changes: SessionMeta) {
        const create = this.views.create;
        const session = create?.session;
        if (!session) {
            return;
        } else if (Object.keys(changes).length === 0) {
            this.hideCreate();
            return;
        }
        if (create) {
            create.creating = true;
        }
        try {
            const updated = await updateSession(session.id, changes);
            if (this.session?.id === updated.id) {
                this.session = updated;
            }
            this.hideCreate();
            this.render();
            void this.refresh();
        } catch (error) {
            this.notifications.error("The chat couldn't be changed", error);
            if (create) {
                create.creating = false;
            }
        }
    }

    private hideCreate() {
        if (this.views.create) {
            this.views.create.visible = false;
            this.views.create.creating = false;
        }
    }

    // create creates the session, and opens it
    private async create(meta: SessionMeta) {
        const create = this.views.create;
        if (create) {
            create.creating = true;
        }
        try {
            const session = await createSession(meta);
            this.hideCreate();
            this.sessions = [session, ...(this.sessions ?? [])];
            location.hash = routeHref('chat', session.id);
        } catch (error) {
            this.notifications.error("The chat couldn't be started", error);
            if (create) {
                create.creating = false;
            }
        }
    }

    // delete deletes the session which is open, once it's confirmed
    private async delete() {
        const session = this.session;
        if (!session || !confirm('Delete this chat, and its messages?')) {
            return;
        }
        try {
            await deleteSession(session.id);
            this.storage.delete('chat', session.id);
            this.sessions = this.sessions?.filter((other) => other.id !== session.id);
            location.hash = routeHref('chat');
        } catch (error) {
            this.notifications.error("The chat couldn't be deleted", error);
        }
    }

    // listed returns the sessions which are listed, with the session which is
    // open, last, when it isn't one of them, such as an older chat, beyond
    // those which are listed, once it's loaded, so it has its actions
    private listed(): Session[] | undefined {
        const { sessions, session } = this;
        if (!sessions || !session || session.serial === 0 || sessions.some((other) => other.id === session.id)) {
            return sessions;
        }
        return [...sessions, session];
    }

    private render() {
        const { page, list, view, details } = this.views;
        if (page) {
            page.heading = this.session?.title || this.session?.model || 'Chat';
            page.description = this.describe();
        }
        if (details) {
            details.message = this.selected;
            details.from = this.selected && isUser(this.selected)
                ? this.details.name?.declined ? 'You' : this.details.name?.value ?? 'You'
                : this.details.assistant?.declined ? 'Assistant' : this.details.assistant?.value ?? (this.session?.model ?? 'Assistant');
        }
        if (list) {
            list.sessions = this.listed();
            list.selected = this.session?.id;
            list.unavailable = this.unavailable;
        }
        if (view) {
            view.session = this.session;
            view.messages = this.messages;
            view.logs = (result, position) => this.session ? this.logs.get(logKey(this.session.id, result, position)) : undefined;
            view.selected = this.selected;
            view.details = this.details;
            const pending = this.session && this.pending.get(this.session.id);
            view.question = pending?.question;
            view.streamed = pending?.streamed ?? [];
            view.progress = pending?.progress;
            view.attachments = this.session ? this.resources.attachments(this.session.id) : [];
            view.unavailable = this.unavailable;
        }
    }
}

// Pending is a question which is waiting for its reply: the question, the
// reply, as it's streamed, how a tool is getting on, how many messages its
// chat had when it was asked, and whether its chat changed meanwhile, so
// it's loaded again, once the reply is stored
interface Pending {
    question: string;
    streamed: Piece[];
    progress?: string;
    earlier: number;
    stale: boolean;
}

// logKey returns the key of the lines tools logged, in the session, by the
// identifier of the result of a tool, or else by the position of a message
export function logKey(session: string, result: string | undefined, position: number): string {
    return result ? `${session}/result/${result}` : `${session}/position/${position}`;
}
