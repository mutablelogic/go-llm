// The chat controller, which shows the sessions in the session list, and the
// conversation of the one which is open in the chat view, sends questions,
// and shows the replies as they're streamed, says someone is typing, starts
// new chats, and deletes them. It loads them again when they change, such as
// when another window replies in a session, and the names of the person and
// the assistant, from the memory, when they change.

import { chat, createSession, deleteSession, getSession, listMessages, listSessions, setStatus, updateSession } from '../api/session';
import { getDetails } from '../api/memory';
import { listModels } from '../api/model';
import { cannot } from '../schema/model';
import { ApiError } from '../api/client';
import { routeHref } from '../router';
import { sessionTokens, type Message, type Session, type SessionMeta } from '../schema/session';
import type { Details } from '../schema/memory';
import type { NotificationController } from './notifications';
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

    // The question which is waiting for its reply, and the reply, as it's
    // streamed, and how a tool is getting on
    private question?: string;
    private streamed: Piece[] = [];
    private progress?: string;

    // Whether the session is to be loaded again, once the reply is stored,
    // as it changed while it was waited for, and when the server was last
    // told someone is typing
    private stale = false;
    private typed = 0;

    // Whether the server can't be reached
    private unavailable = false;

    constructor(views: ChatViews, notifications: NotificationController) {
        this.views = views;
        this.notifications = notifications;
        views.list?.addEventListener(SessionList.eventSelect, (e) => {
            location.hash = routeHref('chat', (e as CustomEvent<Session>).detail.id);
        });
        views.page?.addEventListener(ChatPage.eventNew, () => void this.newChat());
        views.page?.addEventListener(ChatPage.eventDelete, () => void this.delete());
        views.page?.addEventListener(ChatPage.eventEdit, () => void this.edit());
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
        if (id === this.session?.id && this.messages) {
            return;
        }
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
            const changed = !this.messages || session.serial !== this.session.serial;
            this.session = session;
            if (this.question !== undefined) {
                this.stale ||= changed;
            } else if (changed) {
                const messages = (await listMessages(id)).body ?? [];
                if (this.session?.id === id) {
                    this.messages = messages;
                }
            }
        } catch (error) {
            if (error instanceof ApiError && error.code === 404) {
                this.notifications.push({ kind: 'info', title: 'The chat was deleted' });
                location.hash = routeHref('chat');
            } else {
                this.notifications.error("The chat couldn't be loaded", error);
            }
        }
        this.render();
    }

    // send sends the question, and shows the reply as it's streamed, and
    // then adds it to the conversation, or puts the question back, when it
    // couldn't be sent
    private async send(text: string) {
        const session = this.session;
        if (!session || !this.messages || this.question !== undefined) {
            return;
        }
        this.question = text;
        this.streamed = [];
        this.progress = undefined;
        this.render();
        try {
            const response = await chat(session.id, { role: 'user', content: [{ text }] }, {
                onText: (piece) => this.add({ what: 'text', text: piece }),
                onThinking: (piece) => this.add({ what: 'thinking', text: piece }),
                onLog: (line) => this.add({ what: 'log', text: line }),
                onProgress: (progress) => {
                    this.progress = progress.message || progress.status;
                    this.render();
                },
            });
            if (this.session?.id === session.id) {
                const added = response.messages ?? [];
                this.keepLogs(session.id, this.messages.length, added);
                this.messages = [...this.messages, ...added];
                this.session = { ...this.session, serial: response.serial };
            }
        } catch (error) {
            this.notifications.error("The question couldn't be sent", error);
            this.views.view?.restore(text);
            this.stale = true;
        }
        this.question = undefined;
        this.streamed = [];
        this.progress = undefined;
        this.typed = 0;
        this.render();
        if (this.stale && this.session?.id === session.id) {
            this.stale = false;
            this.messages = undefined;
            await this.load(session.id);
        }
        void this.refresh();
    }

    // keepLogs keeps the lines tools logged, as the reply was streamed, by
    // the result of the first tool it called, or else the position of its
    // last message, of the messages which were added after the earlier ones
    private keepLogs(id: string, earlier: number, added: Message[]) {
        const lines = this.streamed.filter((piece) => piece.what === 'log').map((piece) => piece.text.trim());
        if (lines.length === 0) {
            return;
        }
        const result = added.flatMap((message) => message.content).find((block) => block.tool_result?.id)?.tool_result?.id;
        const key = logKey(id, result, earlier + added.length - 1);
        this.logs.set(key, [...(this.logs.get(key) ?? []), ...lines]);
    }

    // describe returns what's said of the chat which is open, beneath its
    // heading: the tokens of its conversation, and whether someone is
    // typing, or its model is replying, or how to start one, when none is
    private describe(): string {
        const session = this.session;
        if (!session) {
            return 'Choose a chat, or start a new one';
        }
        const parts: string[] = [];
        const tokens = sessionTokens(session);
        if (tokens) {
            parts.push(`${tokens.toLocaleString()} tokens`);
        }
        if (this.question !== undefined || session.status === 'replying') {
            parts.push('replying…');
        } else if (session.status === 'typing') {
            parts.push('typing…');
        }
        return parts.join(' · ');
    }

    // add adds a piece of the reply, as it's streamed
    private add(piece: Piece) {
        this.streamed = [...this.streamed, piece];
        this.progress = undefined;
        this.render();
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
            const models = (await listModels(0, 100)).body ?? [];
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
        create.session = this.session;
        create.model = this.session.model;
        create.models = undefined;
        create.visible = true;
        try {
            const models = (await listModels(0, 100)).body ?? [];
            create.models = models.filter((model) => !cannot(model, 'completion') || model.canonical === this.session?.model);
        } catch (error) {
            create.models = [];
            this.notifications.error("Models couldn't be loaded", error);
        }
    }

    // save changes the settings of the chat which is open, when they've
    // changed, and shows it as it is
    private async save(changes: SessionMeta) {
        const session = this.session;
        const create = this.views.create;
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
            this.session = await updateSession(session.id, changes);
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
            this.sessions = this.sessions?.filter((other) => other.id !== session.id);
            location.hash = routeHref('chat');
        } catch (error) {
            this.notifications.error("The chat couldn't be deleted", error);
        }
    }

    private render() {
        const { page, list, view, details } = this.views;
        if (page) {
            page.heading = this.session?.model ?? 'Chat';
            page.description = this.describe();
            page.deletable = this.session !== undefined;
        }
        if (details) {
            details.message = this.selected;
            details.from = this.selected?.role === 'user'
                ? this.details.name?.declined ? 'You' : this.details.name?.value ?? 'You'
                : this.details.assistant?.declined ? 'Assistant' : this.details.assistant?.value ?? (this.session?.model ?? 'Assistant');
        }
        if (list) {
            list.sessions = this.sessions;
            list.selected = this.session?.id;
            list.unavailable = this.unavailable;
        }
        if (view) {
            view.session = this.session;
            view.messages = this.messages;
            view.logs = (result, position) => this.session ? this.logs.get(logKey(this.session.id, result, position)) : undefined;
            view.selected = this.selected;
            view.details = this.details;
            view.question = this.question;
            view.streamed = this.streamed;
            view.progress = this.progress;
            view.unavailable = this.unavailable;
        }
    }
}

// logKey returns the key of the lines tools logged, in the session, by the
// identifier of the result of a tool, or else by the position of a message
export function logKey(session: string, result: string | undefined, position: number): string {
    return result ? `${session}/result/${result}` : `${session}/position/${position}`;
}
