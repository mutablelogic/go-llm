// Carbon theme tokens and type, and the IBM Plex fonts
import '@carbon/styles/css/styles.min.css';
import './fonts.css';

// Carbon UI shell: header and side navigation
import '@carbon/web-components/es/components/ui-shell/index.js';

// Components and pages, which are imported for their elements, which they
// define as they're loaded, so a component which is only used as a type is
// imported for it too, as its type's import is left out of the bundle
import './component/side-nav-footer';
import type { SideNavFooter } from './component/side-nav-footer';
import './component/session-list';
import './component/chat-view';
import './component/new-session';
import './component/message-view';
import './component/resource-panel';
import './component/media-player';
import { MediaPlayer, type Media } from './component/media-player';
import './component/event-view';
import './component/count-filter';
import './component/provider-alerts';
import { CountFilter } from './component/count-filter';
import './component/model-table';
import './component/model-view';
import './component/provider-form';
import './component/model-load';
import { ModelView } from './component/model-view';
import './component/tool-table';
import './component/tool-view';
import './component/agent-editor';
import './component/connector-form';
import './page/chat-page';
import './page/models-page';
import './page/tools-page';
import './page/not-found-page';

// Controllers
import { config, latestConfig } from './api/client';
import { EventsController } from './controller/events';
import { ChatController } from './controller/chat';
import { ModelsController } from './controller/models';
import { ToolsController } from './controller/tools';
import { NotificationController } from './controller/notifications';
import { OutcomesController } from './controller/outcomes';
import { StorageController } from './controller/storage';
import { ResourcesController } from './controller/resources';
import { SessionList } from './component/session-list';
import { Router, routeHref } from './router';

// Errors, and other notifications, are shown as toasts
const notifications = new NotificationController(document.querySelector<HTMLElement>('#notifications')!);

// The defaults of the UI, which are kept in the browser, such as the chat
// which was last open
const storage = new StorageController();

// Show the server's version, and a link to the API's documentation, in the
// side navigation's footer, once the configuration is read
config().then((config) => {
    const footer = document.querySelector<SideNavFooter>('llm-side-nav-footer');
    if (footer) {
        footer.config = config;
    }
}).catch((error) => {
    notifications.error("The configuration couldn't be read", error);
});

// The resources of the chat which is open, in a panel, which its add action
// opens, and those which are attached to its next message
const resources = new ResourcesController(document.querySelector('llm-resource-panel'), notifications);

// The chats: the sessions, the conversation of the one which is open, and the
// panel which starts a new one
const chat = new ChatController({
    page: document.querySelector('llm-chat-page'),
    list: document.querySelector('llm-session-list'),
    view: document.querySelector('llm-chat-view'),
    create: document.querySelector('llm-new-session'),
    details: document.querySelector('llm-message-view'),
}, notifications, storage, resources);

// The outcomes of the events of the chat which is open, such as heartbeats
// which fired, in a panel, which its Events action opens
const outcomes = new OutcomesController(document.querySelector('llm-event-view'), notifications);
document.querySelector('llm-session-list')?.addEventListener(SessionList.eventEvents, () => {
    const session = chat.sessionID();
    if (session) {
        outcomes.open(session);
    }
});

// The models, and the tools, each in a table, with the one which is clicked
// in a panel
const models = new ModelsController({
    page: document.querySelector('llm-models-page'),
    form: document.querySelector('llm-provider-form'),
    load: document.querySelector('llm-model-load'),
    alerts: document.querySelector('llm-provider-alerts'),
    providers: document.querySelector('#provider-filter'),
    table: document.querySelector('llm-model-table'),
    view: document.querySelector('llm-model-view'),
}, notifications);
const tools = new ToolsController({
    page: document.querySelector('llm-tools-page'),
    namespaces: document.querySelector('#namespace-filter'),
    table: document.querySelector('llm-tool-table'),
    view: document.querySelector('llm-tool-view'),
    agent: document.querySelector('llm-agent-editor'),
    connector: document.querySelector('llm-connector-form'),
}, notifications);

// Watch the server's changes, so the pages load what changed: sessions, as
// they're created, changed, replied to, or their status changes, the
// providers, whose models change, the tools, and agents, and connectors,
// as they're connected, and disconnected, the memory's
// details, which are the names of the person and the assistant, and the
// heartbeats, which the UI calls reminders, and events' outcomes. Every
// listener is called once the changes are watched, as they may have changed
// before then.
const events = new EventsController(notifications)
    .on({ schema: 'llm', table: 'session' }, () => void chat.refresh())
    .on({ schema: 'llm', table: 'provider' }, () => void models.refresh())
    .on({ schema: 'llm', table: 'tool' }, () => void tools.refresh())
    .on({ schema: 'mcp', table: 'connector' }, () => void tools.refresh())
    .on({ schema: 'resource', table: 'resource' }, (event) => {
        void resources.refresh();

        // A resource which is deleted isn't played
        if (event?.action === 'delete' && event.row) {
            const session = (event.data as { session?: string } | undefined)?.session;
            document.querySelector('llm-media-player')?.deleted(event.row, session);
        }
    })
    .on({ schema: 'memory', table: 'detail' }, () => void chat.refreshDetails())
    .on({ schema: 'heartbeat', table: 'heartbeat' }, () => void outcomes.refresh())
    .on({ schema: 'llm', table: 'event' }, (event) => {
        void outcomes.refresh();

        // The Events action of the chat which is open is red for a few
        // seconds, when an event for it is created
        const session = (event?.data as { session?: string } | undefined)?.session;
        if (event?.action === 'create' && session && session === chat.sessionID()) {
            document.querySelector('llm-session-list')?.alert();
        }
    })
    .onShutdown(() => chat.stopReplies())
    .onDisconnect(() => {
        chat.disconnect();
        models.disconnect();
        tools.disconnect();
    })
    .onConnect(() => {
        chat.connect();
        models.connect();
        tools.connect();

        // The server's version is read again, as it may have been updated
        // while it was down, which the footer warns of
        latestConfig().then((latest) => {
            const footer = document.querySelector<SideNavFooter>('llm-side-nav-footer');
            if (footer) {
                footer.latest = latest;
            }
        }).catch(() => {
            // It's read again when the server is next reached
        });
    });
// The panel which starts a new chat chooses its tools from them, as they are
tools.onChange((all, namespaces) => {
    const create = document.querySelector('llm-new-session');
    if (create) {
        create.tools = all;
        create.namespaces = namespaces;
    }
});
// A provider which is clicked shows only its models, which the location
// says, such as #models/ollama, so it's linked to, and the back button
// shows every model again
document.querySelector('#provider-filter')?.addEventListener(CountFilter.eventSelect, (e) => {
    const provider = (e as CustomEvent<string | undefined>).detail;
    location.hash = provider ? routeHref('models', provider) : routeHref('models');
});

// A namespace which is clicked shows only its tools, such as #tools/memory
document.querySelector('#namespace-filter')?.addEventListener(CountFilter.eventSelect, (e) => {
    const namespace = (e as CustomEvent<string | undefined>).detail;
    location.hash = namespace ? routeHref('tools', namespace) : routeHref('tools');
});
// A model's new chat action opens the Chat page, and the panel to start a
// new chat, with the model chosen
document.querySelector('llm-model-view')?.addEventListener(ModelView.eventChat, (e) => {
    models.close();
    location.hash = routeHref('chat');
    void chat.newChat((e as CustomEvent<string>).detail);
});
// Audio, and video, such as speech a tool returned, are played in the
// player, in the side navigation, rather than in a new tab
document.addEventListener(MediaPlayer.eventPlay, (e) => {
    document.querySelector('llm-media-player')?.play((e as CustomEvent<Media>).detail);
});
document.addEventListener(MediaPlayer.eventDeleted, (e) => {
    const { hash, session } = (e as CustomEvent<{ hash: string; session?: string }>).detail;
    document.querySelector('llm-media-player')?.deleted(hash, session);
});
events.start();
void models.load();
void tools.load();

// Show the page of the location, such as #chat/<id>, and mark its link in
// the side navigation as active
const router = new Router(document.querySelector<HTMLElement>('#not-found-page')!, 'chat')
    .add('chat', document.querySelector<HTMLElement>('#chat-page'), (route) => void chat.open(route.params[0] || undefined))
    .add('models', document.querySelector<HTMLElement>('#models-page'), (route) => {
        models.setProvider(route.params[0] || undefined);
        void models.refresh();
    })
    .add('tools', document.querySelector<HTMLElement>('#tools-page'), (route) => {
        tools.setNamespace(route.params[0] || undefined);
        void tools.refresh();
    })
    .listen((route) => {
        // A page's panels are closed, once it isn't shown, so they don't stay
        // open over the page which is
        if (route.page !== 'chat') {
            chat.close();
            outcomes.close();
        }
        if (route.page !== 'models') {
            models.close();
        }
        if (route.page !== 'tools') {
            tools.close();
        }
        document.querySelectorAll('cds-side-nav-link').forEach((link) => {
            link.toggleAttribute('active', link.getAttribute('href') === routeHref(route.page));
        });
    });
router.start();
