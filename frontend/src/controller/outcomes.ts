// The controller of the panel of the outcomes of a chat's events, such as
// heartbeats which fired: it opens it for a session, loads its outcomes, the
// most recent first, loads them again when they change, and closes it

import { listEventOutcomes } from '../api/outcome';
import type { EventView } from '../component/event-view';
import { Panel } from '../component/panel';
import type { NotificationController } from './notifications';

export class OutcomesController {
    // The most outcomes which are shown
    static readonly limit = 50;

    private readonly view: EventView | null;
    private readonly notifications: NotificationController;

    // The request which is loading the outcomes, which is aborted when
    // another starts, or the panel closes
    private request?: AbortController;

    constructor(view: EventView | null, notifications: NotificationController) {
        this.view = view;
        this.notifications = notifications;
        view?.addEventListener(Panel.eventClose, () => this.close());
    }

    // open opens the panel, for the session, and loads its outcomes
    open(session: string) {
        if (!this.view) {
            return;
        }
        this.view.session = session;
        this.view.outcomes = undefined;
        this.view.error = undefined;
        void this.refresh();
    }

    // close closes the panel
    close() {
        this.request?.abort();
        this.request = undefined;
        if (this.view) {
            this.view.session = undefined;
        }
    }

    // refresh loads the outcomes of the session whose panel is open, as
    // they may have changed
    async refresh() {
        const session = this.view?.session;
        if (!this.view || !session) {
            return;
        }
        this.request?.abort();
        const request = new AbortController();
        this.request = request;
        try {
            const list = await listEventOutcomes(session, OutcomesController.limit, request.signal);
            if (request.signal.aborted || this.view.session !== session) {
                return;
            }
            this.view.outcomes = list.body ?? [];
            this.view.count = list.count;
            this.view.error = undefined;
        } catch (error) {
            if (request.signal.aborted) {
                return;
            }
            this.view.error = error instanceof Error ? error.message : String(error);
            this.notifications.error("The chat's events couldn't be loaded", error);
        }
    }
}
