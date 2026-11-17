// The Conversations module: the chats, their resources, dictation, and the
// outcomes of their events, such as heartbeats which fired

import { ChatController } from '../controller/chat';
import { DictationController } from '../controller/dictation';
import { OutcomesController } from '../controller/outcomes';
import { ResourcesController } from '../controller/resources';
import { ChatPage } from '../page/chat-page';
import { addPage, Section, type App } from './app';

// ChatModule is what the other modules use of the Conversations module
export interface ChatModule {
    chat: ChatController;
    dictation: DictationController;
    outcomes: OutcomesController;
}

// chatModule adds the Conversations page, at #chat, or #chat/<id>, to the app
export function chatModule(app: App): ChatModule {
    const page = addPage<ChatPage>(app, 'llm-chat-page', 'chat-page', `
        <div class="chat">
            <llm-session-list></llm-session-list>
            <llm-chat-view></llm-chat-view>
        </div>
        <llm-new-session></llm-new-session>
        <llm-message-view></llm-message-view>
        <llm-resource-panel></llm-resource-panel>
        <llm-event-view></llm-event-view>
    `);

    // The resources of the chat which is open, in a panel, which its add
    // action opens, and those which are attached to its next message
    const resources = new ResourcesController(page.querySelector('llm-resource-panel'), app.notifications);

    // The chats: the sessions, the conversation of the one which is open,
    // and the panel which starts a new one
    const chat = new ChatController({
        page,
        list: page.querySelector('llm-session-list'),
        view: page.querySelector('llm-chat-view'),
        create: page.querySelector('llm-new-session'),
        details: page.querySelector('llm-message-view'),
    }, app.notifications, app.storage, resources);

    // Dictation of a message, into the chat's box, when the browser
    // recognises speech
    const dictation = new DictationController(page.querySelector('llm-chat-view'), app.notifications);

    // The outcomes of the events of the chat which is open, such as
    // heartbeats which fired, in a panel, which the Conversation menu's
    // Tasks opens
    const outcomes = new OutcomesController(page.querySelector('llm-event-view'), app.notifications);
    page.addEventListener(ChatPage.eventTasks, () => {
        const session = chat.sessionID();
        if (session) {
            outcomes.open(session);
        }
    });

    // The sessions, as they're created, changed, replied to, or their status
    // changes, their resources, the memory's details, which are the names of
    // the person and the assistant, and the heartbeats, which the UI calls
    // reminders, and events' outcomes
    app.changes
        .on({ schema: 'llm', table: 'session' }, () => void chat.refresh())
        .on({ schema: 'llm', table: 'provider' }, () => void chat.refreshModels())
        .on({ schema: 'resource', table: 'resource' }, (change) => {
            void resources.refresh();

            // A resource which is deleted isn't played
            if (change?.action === 'delete' && change.row) {
                const session = (change.data as { session?: string } | undefined)?.session;
                document.querySelector('llm-media-player')?.deleted(change.row, session);
            }
        })
        .on({ schema: 'memory', table: 'detail' }, () => void chat.refreshDetails())
        .on({ schema: 'heartbeat', table: 'heartbeat' }, () => void outcomes.refresh())
        .on({ schema: 'llm', table: 'event' }, (change) => {
            void outcomes.refresh();

            // The Conversation menu is marked for a few seconds, when an
            // event for the chat which is open is created
            const session = (change?.data as { session?: string } | undefined)?.session;
            if (change?.action === 'create' && session && session === chat.sessionID()) {
                page.alert();
            }
        })
        .onShutdown(() => chat.stopReplies())
        .onDisconnect(() => chat.disconnect())
        .onConnect(() => chat.connect());

    // Its page, in the sidebar, and its route, whose parameter is the chat
    // which is open
    app.nav.setModule('chat', Section, [{ id: 'chat', label: 'Conversations', href: '#chat' }]);
    // A page which isn't found shows Conversations, whose parameters aren't
    // a chat's
    app.router.add('chat', page, (route) => void chat.open(route.page === 'chat' ? route.params[0] || undefined : undefined));
    app.router.onRoute((route) => {
        // Dictation stops, as the page, or the chat, changes, so what's said
        // isn't put in another's box, and its panels are closed, once it
        // isn't shown, so they don't stay open over the page which is
        dictation.stop();
        if (route.page !== 'chat') {
            chat.close();
            outcomes.close();
        }
    });
    void chat.refreshModels();
    return { chat, dictation, outcomes };
}
