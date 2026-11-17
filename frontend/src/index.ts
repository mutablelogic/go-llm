// Carbon theme tokens and type, and the IBM Plex fonts
import '@carbon/styles/css/styles.min.css';
import './fonts.css';

// go-server's shell: the header, and the sidebar of the modules' pages, the
// server's status, below them, and the notifications
import { ChangesController } from '@mutablelogic/go-server/broadcaster';
import { config } from './api/client';
import { ConfigController, configClient, getConfig } from '@mutablelogic/go-server/config';
import '@mutablelogic/go-server/config/view';
import { NavController } from '@mutablelogic/go-server/nav';
import '@mutablelogic/go-server/nav/view';
import { NotificationController } from '@mutablelogic/go-server/notification';
import '@mutablelogic/go-server/notification/view';
import { Router } from '@mutablelogic/go-server/router';

// Components and pages, which are imported for their elements, which they
// define as they're loaded, so a component which is only used as a type is
// imported for it too, as its type's import is left out of the bundle
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
import './component/model-table';
import './component/model-view';
import './component/provider-form';
import './component/model-load';
import { ModelView } from './component/model-view';
import './component/tool-table';
import './component/tool-view';
import './component/agent-editor';
import './component/connector-form';
import './component/connector-alerts';
import './page/chat-page';
import './page/models-page';
import './page/tools-page';

// Controllers, and the modules, each of which adds its pages
import { StorageController } from './controller/storage';
import { routeHref } from './router';
import type { App } from './module/app';
import { chatModule } from './module/chat';
import { modelsModule } from './module/models';
import { toolsModule } from './module/tools';

// Errors, and other notifications, are shown as toasts
const notifications = new NotificationController();
const toasts = document.querySelector('gs-notifications');
if (toasts) {
    toasts.controller = notifications;
}

// The configuration says where the API is, and the changes, which the
// server's status, in the sidebar, follows, as it goes down, comes back,
// and is upgraded
const read = await getConfig().catch((error) => {
    notifications.error("The configuration couldn't be read", error);
    return undefined;
});
const changes = new ChangesController(read ? configClient(read) : {
    // Without the configuration, the API's prefix is read as each stream
    // is opened, until it's read, so a server with another prefix, which
    // couldn't be reached when the page was loaded, is watched once it can
    // be, as the stream is opened again
    base: '',
    fetch: async (input, init) => fetch((await config()).prefix + String(input), init),
})
    .onError((error) => notifications.error("Changes couldn't be watched", error));
const status = document.querySelector('gs-server-status');
if (status) {
    status.controller = new ConfigController(changes, { config: read, notifications });
    status.repository = 'https://github.com/mutablelogic/go-llm';
}

// The navigation, whose sidebar has the modules' pages, and the router of
// the pages, which shows Conversations when the location has no page
const nav = new NavController({ name: 'llm', prefix: 'go', href: '#', pages: [] });
const header = document.querySelector('gs-header');
const sideNav = document.querySelector('gs-side-nav');
if (header && sideNav) {
    header.controller = nav;
    sideNav.controller = nav;
}
const router = new Router('chat');

// The modules, which share the UI's defaults, which are kept in the browser,
// such as the chat which was last open
const app: App = { nav, router, root: document.querySelector('main')!, notifications, changes, storage: new StorageController() };
const { chat } = chatModule(app);
const models = modelsModule(app);
const tools = toolsModule(app);

// The panel which starts a new chat chooses its tools from them, as they are
tools.onChange((all, namespaces) => {
    const create = document.querySelector('llm-new-session');
    if (create) {
        create.tools = all;
        create.namespaces = namespaces;
    }
});

// A model's new chat action opens the Conversations page, and the panel to
// start a new chat, with the model chosen
document.querySelector('llm-model-view')?.addEventListener(ModelView.eventChat, (e) => {
    models.close();
    location.hash = routeHref('chat');
    void chat.newChat((e as CustomEvent<string>).detail);
});

// Audio, and video, such as speech a tool returned, are played in the
// player, in the sidebar, rather than in a new tab
document.addEventListener(MediaPlayer.eventPlay, (e) => {
    document.querySelector('llm-media-player')?.play((e as CustomEvent<Media>).detail);
});
document.addEventListener(MediaPlayer.eventDeleted, (e) => {
    const { hash, session } = (e as CustomEvent<{ hash: string; session?: string }>).detail;
    document.querySelector('llm-media-player')?.deleted(hash, session);
});

// Show the page of the location, such as #chat/<id>, which is active in
// the sidebar, and follow the server's changes
router.onRoute((route) => nav.setActive(route.page)).start();
changes.start();
