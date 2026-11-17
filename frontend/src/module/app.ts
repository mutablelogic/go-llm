// The app which modules are added to: the navigation, whose sidebar has each
// module's pages, the router of their pages, the element the pages are in,
// the notifications, the server's changes, and the UI's defaults, which are
// kept in the browser

import type { ChangesController } from '@mutablelogic/go-server/broadcaster';
import type { NavController } from '@mutablelogic/go-server/nav';
import type { NotificationController } from '@mutablelogic/go-server/notification';
import type { Router } from '@mutablelogic/go-server/router';
import type { StorageController } from '../controller/storage';

export interface App {
    nav: NavController;
    router: Router;
    root: HTMLElement;
    notifications: NotificationController;
    changes: ChangesController;
    storage: StorageController;
}

// Section is the section of the sidebar which every module's pages are in,
// so the header has no menu of sections
export const Section = 'llm';

// addPage adds a page, of the element's markup, to the app's root, hidden
// until it's routed to, and returns it
export function addPage<T extends HTMLElement>(app: App, tag: string, id: string, markup: string): T {
    const page = document.createElement(tag) as T;
    page.id = id;
    page.className = 'full';
    page.hidden = true;
    page.innerHTML = markup;
    app.root.append(page);
    return page;
}
