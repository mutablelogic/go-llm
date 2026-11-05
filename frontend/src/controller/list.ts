// A controller of a list which the API returns a page at a time, such as
// the models, or the tools, which loads a page, and shows it in the views it
// controls, which a subclass renders: a table, with pagination, and a view,
// which shows the item which is clicked in the table, until it's closed. It
// loads the page again when what's in it changes, keeping what's shown
// until it's loaded, and keeps the item which is shown current. When the
// page can't be loaded, the views keep what they showed, or are empty, and
// a notification says why.

import { ApiError } from '../api/client';
import type { NotificationController } from './notifications';

// Page is a page of a list, with the number of items in the list
export interface Page<T> {
    count: number;
    body?: T[];
}

export abstract class ListController<T> {
    protected readonly notifications: NotificationController;

    // The name of the items, such as Models, in the notification which says
    // why they can't be loaded
    private readonly name: string;

    // The items on the page, which are undefined while they're first
    // loading, the page, from 1, its size, and the number of items
    protected items?: T[];
    protected page = 1;
    protected pageSize = 25;
    protected total = 0;

    // The item which is shown in the view, or none
    protected selected?: T;

    // Whether the server can't be reached, when every item is unavailable,
    // until it can be again
    protected unavailable = false;

    // The refresh in progress, which isn't cancelled by another, so a burst
    // of changes doesn't stop the items loading, and whether to refresh
    // again once it's done, as they changed while it loaded them
    private refreshing?: Promise<void>;
    private again = false;

    constructor(name: string, notifications: NotificationController) {
        this.name = name;
        this.notifications = notifications;
    }

    // list returns the page of the items, at the offset, of at most the limit
    protected abstract list(offset: number, limit: number): Promise<Page<T>>;

    // get returns the item again, by itself
    protected abstract get(item: T): Promise<T>;

    // same returns true when the items are the same, by their keys
    protected abstract same(a: T, b: T): boolean;

    // render shows the items, the page, and the item which is selected, in
    // the views
    protected abstract render(): void;

    // load shows the views loading, and requests the page, and shows it in
    // the views, or none, when it can't be loaded
    load(): Promise<void> {
        this.items = undefined;
        this.render();
        return this.refresh();
    }

    // refresh requests the page again, keeping the items which are shown
    // until they're loaded. A page which is beyond the last, as items were
    // deleted, is the last. While a refresh is in progress, it's refreshed
    // once more when it's done.
    refresh(): Promise<void> {
        if (this.refreshing) {
            this.again = true;
            return this.refreshing;
        }
        this.refreshing = (async () => {
            do {
                this.again = false;
                try {
                    const list = await this.list((this.page - 1) * this.pageSize, this.pageSize);
                    this.total = list.count;
                    const last = Math.max(1, Math.ceil(list.count / this.pageSize));
                    if (this.page > last) {
                        this.page = last;
                        this.again = true;
                        continue;
                    }
                    this.items = list.body ?? [];
                    await this.reloadSelected();
                } catch (error) {
                    this.items = this.items ?? [];
                    this.notifications.error(`${this.name} couldn't be loaded`, error);
                }
                this.render();
            } while (this.again);
            this.refreshing = undefined;
        })();
        return this.refreshing;
    }

    // setPage loads the page, from 1
    setPage(page: number) {
        this.page = page;
        void this.refresh();
    }

    // setPageSize loads the first page of the size
    setPageSize(size: number) {
        this.pageSize = size;
        this.page = 1;
        void this.refresh();
    }

    // open shows the item in the view, and selects it in the table
    open(item: T) {
        this.selected = item;
        this.render();
    }

    // close closes the view, and the item is no longer selected
    close() {
        if (this.selected) {
            this.selected = undefined;
            this.render();
        }
    }

    // disconnect shows every item as unavailable, as the server can't be
    // reached, until it can be again
    disconnect() {
        this.unavailable = true;
        this.render();
    }

    // connect shows the items as they are again, as the server can be
    // reached
    connect() {
        this.unavailable = false;
        this.render();
    }

    // reloadSelected loads the item which is shown again, from the page,
    // when it's on it, or else by itself, as it can have moved to another
    // page, and closes the view when it's been deleted
    private async reloadSelected() {
        const selected = this.selected;
        if (selected === undefined) {
            return;
        }
        const item = this.items?.find((item) => this.same(item, selected));
        if (item) {
            this.selected = item;
            return;
        }
        try {
            const item = await this.get(selected);
            if (this.selected && this.same(this.selected, selected)) {
                this.selected = item;
            }
        } catch (error) {
            if (!(error instanceof ApiError && error.code === 404)) {
                throw error;
            } else if (this.selected && this.same(this.selected, selected)) {
                this.selected = undefined;
            }
        }
    }
}
