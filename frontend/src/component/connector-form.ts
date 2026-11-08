// A panel which adds a custom connector: the URL of a remote MCP server, and
// the patterns of the names of its tools which are included, and excluded.
// It fires an event with them, as Connect is clicked, and shows that it's
// connecting, and why it couldn't connect, such as that the server can't be
// reached.

import { css, html, nothing, type PropertyValues } from 'lit';
import { keyed } from 'lit/directives/keyed.js';
import { customElement, property, query, state } from 'lit/decorators.js';
import { Panel } from './panel';
import type { ConnectorMeta } from '../schema/connector';

// Carbon components
import '@carbon/web-components/es/components/textarea/index.js';
import '@carbon/web-components/es/components/text-input/index.js';
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/notification/inline-notification.js';

// ConnectorFormItem is what the panel shows, which is a connector to add
export type ConnectorFormItem = { mode: 'add' };

@customElement('llm-connector-form')
export class ConnectorForm extends Panel<ConnectorFormItem> {
    // The name of the event fired with the connector to add
    static readonly eventConnect = 'llm-connector-form-connect';

    // What's shown, which is undefined when the panel is closed
    @property({ attribute: false })
    item?: ConnectorFormItem;

    // Whether it's connecting, and why it couldn't, when it couldn't
    @property({ type: Boolean })
    connecting = false;

    @property({ attribute: false })
    error?: string;

    // How many times the panel has been opened, so its form is made again
    // each time
    @state()
    private opened = 0;

    @query('cds-text-input#url')
    private urlInput?: HTMLElement & { value: string };

    @query('cds-textarea#include')
    private includeInput?: HTMLElement & { value: string };

    @query('cds-textarea#exclude')
    private excludeInput?: HTMLElement & { value: string };

    static styles = [Panel.styles, css`
        form > * {
            display: block;
            margin-bottom: 1.5rem;
        }

        .footer {
            position: sticky;
            bottom: -1rem;
            margin: 0 -1rem -1rem;
            padding: 1rem;
            background: var(--cds-layer-01);
            border-top: 1px solid var(--cds-border-subtle-01);
        }
    `];

    // willUpdate starts the form again, each time it opens
    willUpdate(changed: PropertyValues<this>) {
        if (changed.has('item') && this.item && changed.get('item') !== this.item) {
            this.opened++;
            this.error = undefined;
        }
        super.willUpdate(changed);
    }

    protected current(): ConnectorFormItem | undefined {
        return this.item;
    }

    protected renderTitle() {
        return 'Add custom connector';
    }

    protected renderSubtitle() {
        return "A server, by its URL, whose tools the assistant can call, once it's connected";
    }

    protected renderBody() {
        return keyed(this.opened, html`
            <form @submit=${this.handleSubmit} @focusin=${this.clearError}>
                <cds-text-input id="url" label="URL" placeholder="https://mcp.example.com/mcp"
                    helper-text="The URL of the MCP server, of its streamable HTTP, or SSE, transport"></cds-text-input>
                <cds-textarea id="include" label="Include tools (optional)" rows="2"
                    helper-text="Regular expressions of the names of the tools to include, one a line, or every tool"></cds-textarea>
                <cds-textarea id="exclude" label="Exclude tools (optional)" rows="2"
                    helper-text="Regular expressions of the names of the tools to exclude, one a line, such as signup"></cds-textarea>
                ${this.error ? html`<cds-inline-notification kind="error" title="It couldn't connect" subtitle=${this.error} hide-close-button low-contrast></cds-inline-notification>` : nothing}
                <div class="footer">
                    <cds-button kind="primary" type="submit" ?disabled=${this.connecting} @click=${this.handleSubmit}>
                        ${this.connecting ? 'Connecting…' : 'Connect'}
                    </cds-button>
                </div>
            </form>
        `);
    }

    // clearError clears why it couldn't connect, once a field is focused, so
    // it's changed, apart from the button, which is focused as it's clicked
    private clearError(event: FocusEvent) {
        if (this.error && !(event.target instanceof Element && event.target.closest('cds-button'))) {
            this.error = undefined;
        }
    }

    private handleSubmit(event: Event) {
        event.preventDefault();
        if (this.connecting || !this.item) {
            return;
        }
        const url = this.urlInput?.value.trim() ?? '';
        if (!url) {
            this.error = "The server's URL is needed";
            return;
        }
        const meta: ConnectorMeta = {
            url,
            include: lines(this.includeInput?.value),
            exclude: lines(this.excludeInput?.value),
        };
        this.dispatchEvent(new CustomEvent(ConnectorForm.eventConnect, { detail: meta }));
    }
}

// lines returns the lines of the text, without the space around them, apart
// from those which are empty, or undefined, when there are none
function lines(text?: string): string[] | undefined {
    const result = (text ?? '').split('\n').map((line) => line.trim()).filter((line) => line !== '');
    return result.length ? result : undefined;
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-connector-form': ConnectorForm;
    }
}
