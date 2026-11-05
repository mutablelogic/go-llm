// The notification controller, which shows the notifications pushed to it
// as toasts, stacked in its container, until they time out or are closed

import { html, render } from 'lit';
import { repeat } from 'lit/directives/repeat.js';

// Carbon components
import '@carbon/web-components/es/components/notification/toast-notification.js';

// NotificationKind is the kind of a notification, which sets its colour and
// icon
export type NotificationKind = 'error' | 'warning' | 'info' | 'success';

// Notification is a notification which is pushed, which closes after its
// timeout in milliseconds, or the default
export interface Notification {
    kind: NotificationKind;
    title: string;
    subtitle?: string;
    timeout?: number;
}

export class NotificationController {
    // How long a notification is shown, unless it says otherwise
    static readonly defaultTimeout = 8000;

    private readonly container: HTMLElement;

    // The notifications which are shown, by their ids, in the order they
    // were pushed
    private readonly notifications = new Map<number, Notification>();
    private nextId = 0;

    constructor(container: HTMLElement) {
        this.container = container;
    }

    // push shows the notification
    push(notification: Notification) {
        this.notifications.set(this.nextId++, notification);
        this.render();
    }

    // error shows an error, with the title saying what failed, and the
    // error why
    error(title: string, error: unknown) {
        this.push({ kind: 'error', title, subtitle: error instanceof Error ? error.message : String(error) });
    }

    // close removes the notification, once it has closed
    private close(id: number) {
        this.notifications.delete(id);
        this.render();
    }

    private render() {
        render(html`${repeat(this.notifications, ([id]) => id, ([id, notification]) => html`
            <cds-toast-notification kind=${notification.kind} title=${notification.title}
                subtitle=${notification.subtitle ?? ''} timeout=${notification.timeout ?? NotificationController.defaultTimeout}
                @cds-notification-closed=${() => this.close(id)}></cds-toast-notification>
        `)}`, this.container);
    }
}
