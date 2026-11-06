// The controller of the panel of a chat's heartbeats, and the outcomes of
// its events, such as heartbeats which fired: it opens it for a session,
// loads its heartbeats, and its outcomes, the most recent first, loads them
// again when they change, deletes a heartbeat, once it's confirmed, and
// closes it

import { listEventOutcomes } from '../api/outcome';
import { deleteHeartbeat, listHeartbeats } from '../api/heartbeat';
import { EventView } from '../component/event-view';
import type { Heartbeat } from '../schema/heartbeat';
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
        view?.addEventListener(EventView.eventDeleteHeartbeat, (e) => void this.deleteHeartbeat((e as CustomEvent<Heartbeat>).detail));
    }

    // open opens the panel, for the session, and loads its outcomes
    open(session: string) {
        if (!this.view) {
            return;
        }
        this.view.session = session;
        this.view.outcomes = undefined;
        this.view.heartbeats = undefined;
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

    // refresh loads the heartbeats, and the outcomes, of the session whose
    // panel is open, as they may have changed. Its heartbeats aren't shown,
    // when they can't be loaded, such as when the server has none.
    async refresh() {
        const session = this.view?.session;
        if (!this.view || !session) {
            return;
        }
        this.request?.abort();
        const request = new AbortController();
        this.request = request;
        try {
            const [list, heartbeats] = await Promise.all([
                listEventOutcomes(session, OutcomesController.limit, request.signal),
                listHeartbeats(session, request.signal).catch(() => undefined),
            ]);
            if (request.signal.aborted || this.view.session !== session) {
                return;
            }
            this.view.heartbeats = heartbeats && (heartbeats.body ?? []);
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

    // deleteHeartbeat deletes the heartbeat, once it's confirmed, which
    // is then gone from the panel, as it's loaded again, once the server
    // says it's deleted
    private async deleteHeartbeat(heartbeat: Heartbeat) {
        if (!this.view || this.view.deleting.has(heartbeat.id) || !confirm(`Delete the reminder ${heartbeat.title}?`)) {
            return;
        }
        this.view.deleting = new Set([...this.view.deleting, heartbeat.id]);
        try {
            await deleteHeartbeat(heartbeat.id);
            this.notifications.push({ kind: 'success', title: `${heartbeat.title} was deleted` });
            void this.refresh();
        } catch (error) {
            this.notifications.error(`${heartbeat.title} couldn't be deleted`, error);
        } finally {
            const deleting = new Set(this.view.deleting);
            deleting.delete(heartbeat.id);
            this.view.deleting = deleting;
        }
    }
}
