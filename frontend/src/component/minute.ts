// A reactive controller which renders the component it's added to each
// minute, while it's in the page, so the times it shows relative to now,
// such as 5 minutes ago (see formatAgo), are current

import type { ReactiveController, ReactiveControllerHost } from 'lit';

export class MinuteController implements ReactiveController {
    private readonly host: ReactiveControllerHost;
    private timer?: ReturnType<typeof setInterval>;

    constructor(host: ReactiveControllerHost) {
        this.host = host;
        host.addController(this);
    }

    hostConnected() {
        this.timer = setInterval(() => this.host.requestUpdate(), 60 * 1000);
    }

    hostDisconnected() {
        clearInterval(this.timer);
        this.timer = undefined;
    }
}
