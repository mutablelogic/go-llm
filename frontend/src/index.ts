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
import './component/event-view';
import './component/count-filter';
import './component/provider-alerts';
import { CountFilter } from './component/count-filter';
import './component/model-table';
import './component/model-view';
import { ModelView } from './component/model-view';
import './component/tool-table';
import './component/tool-view';
import './page/chat-page';
import './page/models-page';
import './page/tools-page';
import './page/not-found-page';

// Controllers
import { config } from './api/client';
import { EventsController } from './controller/events';
import { ChatController } from './controller/chat';
import { ModelsController } from './controller/models';
import { ToolsController } from './controller/tools';
import { NotificationController } from './controller/notifications';
import { OutcomesController } from './controller/outcomes';
import { ChatPage } from './page/chat-page';
import { Router, routeHref } from './router';
import { eventRefresh } from './page/refresh';

// Errors, and other notifications, are shown as toasts
const notifications = new NotificationController(document.querySelector<HTMLElement>('#notifications')!);

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

// The chats: the sessions, the conversation of the one which is open, and the
// panel which starts a new one
const chat = new ChatController({
    page: document.querySelector('llm-chat-page'),
    list: document.querySelector('llm-session-list'),
    view: document.querySelector('llm-chat-view'),
    create: document.querySelector('llm-new-session'),
    details: document.querySelector('llm-message-view'),
}, notifications);

// The outcomes of the events of the chat which is open, such as heartbeats
// which fired, in a panel, which its Events action opens
const outcomes = new OutcomesController(document.querySelector('llm-event-view'), notifications);
document.querySelector('llm-chat-page')?.addEventListener(ChatPage.eventEvents, () => {
    const session = chat.sessionID();
    if (session) {
        outcomes.open(session);
    }
});

// The models, and the tools, each in a table, with the one which is clicked
// in a panel
const models = new ModelsController({
    alerts: document.querySelector('llm-provider-alerts'),
    providers: document.querySelector('#provider-filter'),
    table: document.querySelector('llm-model-table'),
    view: document.querySelector('llm-model-view'),
}, notifications);
const tools = new ToolsController({
    namespaces: document.querySelector('#namespace-filter'),
    table: document.querySelector('llm-tool-table'),
    view: document.querySelector('llm-tool-view'),
}, notifications);

// Watch the server's changes, so the pages load what changed: sessions, as
// they're created, changed, replied to, or their status changes, the
// providers, whose models change, the tools, and agents, and the memory's
// details, which are the names of the person and the assistant. Every
// listener is called once the changes are watched, as they may have changed
// before then.
const events = new EventsController(notifications)
    .on({ schema: 'llm', table: 'session' }, () => void chat.refresh())
    .on({ schema: 'llm', table: 'provider' }, () => void models.refresh())
    .on({ schema: 'llm', table: 'tool' }, () => void tools.refresh())
    .on({ schema: 'memory', table: 'detail' }, () => void chat.refreshDetails())
    .on({ schema: 'llm', table: 'event' }, () => void outcomes.refresh())
    .onDisconnect(() => {
        chat.disconnect();
        models.disconnect();
        tools.disconnect();
    })
    .onConnect(() => {
        chat.connect();
        models.connect();
        tools.connect();
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
// The models and tools pages' refresh actions load them again
document.querySelector('llm-models-page')?.addEventListener(eventRefresh, () => void models.refresh());
document.querySelector('llm-tools-page')?.addEventListener(eventRefresh, () => void tools.refresh());
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
