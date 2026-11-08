// A panel which adds a custom connector: the URL of a remote MCP server, and
// the patterns of the names of its tools which are included, and excluded,
// or edits one. It fires an event with them, as Connect, or Save, is
// clicked, and shows that it's connecting, and why it couldn't connect, or
// be saved, such as that the server can't be reached.

import { css, html, nothing, type PropertyValues } from 'lit';
import { keyed } from 'lit/directives/keyed.js';
import { customElement, property, query, state } from 'lit/decorators.js';
import { Panel } from './panel';
import { config } from '../api/client';
import { needsSignIn, type Connector, type ConnectorMeta } from '../schema/connector';

// Carbon components
import '@carbon/web-components/es/components/textarea/index.js';
import '@carbon/web-components/es/components/text-input/index.js';
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/notification/inline-notification.js';
import './disclosure';
import '@carbon/web-components/es/components/copy-button/index.js';

// ConnectorFormItem is what the panel shows: a connector to add, or the
// connector which is edited
export type ConnectorFormItem = { mode: 'add' } | { mode: 'edit'; connector: Connector };

@customElement('llm-connector-form')
export class ConnectorForm extends Panel<ConnectorFormItem> {
    // The names of the events fired with the connector to add, and with the
    // changes to the connector which is edited
    static readonly eventConnect = 'llm-connector-form-connect';
    static readonly eventSave = 'llm-connector-form-save';

    // The name of the event fired with the name of the connector which is
    // added, as Sign in is clicked, once its server needs to be signed in to
    static readonly eventSignIn = 'llm-connector-form-sign-in';

    // What's shown, which is undefined when the panel is closed
    @property({ attribute: false })
    item?: ConnectorFormItem;

    // Whether it's connecting, and why it couldn't, when it couldn't
    @property({ type: Boolean })
    connecting = false;

    @property({ attribute: false })
    error?: string;

    // The name of the connector which was added, whose server needs to be
    // signed in to, before it's connected, which is waited for
    @property({ attribute: false })
    signIn?: string;

    // How many times the panel has been opened, so its form is made again
    // each time
    @state()
    private opened = 0;

    // The URL the authorization server redirects back to, once a person has
    // signed in, which a client which is registered beforehand, such as a
    // GitHub OAuth app, is registered with
    @state()
    private redirect?: string;

    @query('cds-text-input#url')
    private urlInput?: HTMLElement & { value: string };

    @query('cds-textarea#include')
    private includeInput?: HTMLElement & { value: string };

    @query('cds-textarea#exclude')
    private excludeInput?: HTMLElement & { value: string };

    @query('cds-text-input#client-id')
    private clientIdInput?: HTMLElement & { value: string };

    @query('cds-text-input#client-secret')
    private clientSecretInput?: HTMLElement & { value: string };

    static styles = [Panel.styles, css`
        form > * {
            display: block;
            margin-bottom: 1.5rem;
        }

        /* The advanced fields are spaced as the form is */
        llm-disclosure > * {
            display: block;
            margin-bottom: 1.5rem;
        }

        .note {
            color: var(--cds-text-secondary);
            font-size: 0.875rem;
            line-height: 1.43;
        }

        .redirect {
            display: flex;
            align-items: flex-end;
            gap: 0.25rem;
        }

        .redirect cds-text-input {
            flex: 1;
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
            this.signIn = undefined;
        }
        super.willUpdate(changed);
    }

    // connectedCallback finds the redirect URL, from the API's prefix, at
    // this page's origin, which the server's callback is beneath
    connectedCallback() {
        super.connectedCallback();
        if (!this.redirect) {
            void config().then(({ prefix }) => {
                this.redirect = new URL(prefix.replace(/\/$/, '') + '/mcp/callback', location.origin).href;
            }).catch(() => undefined);
        }
    }

    protected current(): ConnectorFormItem | undefined {
        return this.item;
    }

    protected renderTitle(item: ConnectorFormItem) {
        return item.mode === 'add' ? 'Add custom connector' : `Edit ${item.connector.namespace ?? item.connector.name}`;
    }

    protected renderSubtitle(item: ConnectorFormItem) {
        return item.mode === 'add'
            ? "A server, by its URL, whose tools the assistant can call, once it's connected"
            : `The connector ${item.connector.name}, to ${item.connector.server?.name ?? item.connector.url}`;
    }

    protected renderBody(item: ConnectorFormItem) {
        const editing = item.mode === 'edit' ? item.connector : undefined;
        const busy = editing ? 'Saving…' : 'Connecting…';

        // The connector which is edited, whose server needs to be signed in
        // to, can be, as well as saved
        const editSignIn = editing && needsSignIn(editing) ? editing.name : undefined;
        return keyed(this.opened, html`
            <form @submit=${this.handleSubmit} @focusin=${this.clearError}>
                <cds-text-input id="url" label="URL" placeholder="https://mcp.example.com/mcp" value=${editing?.url ?? ''}
                    helper-text="The URL of the MCP server, of its streamable HTTP, or SSE, transport"></cds-text-input>
                <cds-textarea id="include" label="Include tools (optional)" rows="2" value=${(editing?.include ?? []).join('\n')}
                    helper-text="Regular expressions of the names of the tools to include, one a line, or every tool"></cds-textarea>
                <cds-textarea id="exclude" label="Exclude tools (optional)" rows="2" value=${(editing?.exclude ?? []).join('\n')}
                    helper-text="Regular expressions of the names of the tools to exclude, one a line, such as signup"></cds-textarea>
                <llm-disclosure label="Advanced">
                        <p class="note">Some servers' sign-in, such as GitHub's, doesn't register clients itself, so you register one with it, with the redirect URL. Its ID and secret are kept by the server, such as in its environment variables, and you give their names here, rather than the ID and secret themselves.</p>
                        <div class="redirect">
                            <cds-text-input id="redirect" label="Redirect URL" readonly value=${this.redirect ?? ''}
                                helper-text="Where the server's sign-in sends the browser back to"></cds-text-input>
                            <cds-copy-button feedback="Copied" @click=${this.copyRedirect}>Copy the redirect URL</cds-copy-button>
                        </div>
                        <cds-text-input id="client-id" label="Client ID credential name (optional)" placeholder="GITHUB_CLIENT_ID" value=${editing?.client_id_credential ?? ''}
                            helper-text="The name of the server's credential, such as an environment variable, which holds the client's ID, not the ID itself"></cds-text-input>
                        <cds-text-input id="client-secret" label="Client secret credential name (optional)" placeholder="GITHUB_CLIENT_SECRET" value=${editing?.client_secret_credential ?? ''}
                            helper-text="The name of the server's credential, such as an environment variable, which holds the client's secret, not the secret itself"></cds-text-input>
                    </llm-disclosure>
                ${this.error ? html`<cds-inline-notification kind="error" title=${editing ? "It couldn't be saved" : "It couldn't connect"} subtitle=${this.error} hide-close-button low-contrast></cds-inline-notification>` : nothing}
                ${this.signIn ? html`<cds-inline-notification kind="info" title=${`Sign in to ${this.signIn}`} subtitle="The server needs you to sign in, in a new tab, before it connects. This panel carries on once you have." hide-close-button low-contrast></cds-inline-notification>` : nothing}
                ${editSignIn ? html`<cds-inline-notification kind="warning" title=${`${editSignIn} needs you to sign in`} subtitle="Its server needs you to sign in, in a new tab, before its tools can be used. Signing in saves your changes first." hide-close-button low-contrast></cds-inline-notification>` : nothing}
                <div class="footer">
                    ${this.signIn
                        ? html`<cds-button kind="primary" type="button" @click=${this.handleSignIn}>Sign in</cds-button>`
                        : html`<cds-button kind="primary" type="submit" ?disabled=${this.connecting} @click=${this.handleSubmit}>
                            ${this.connecting ? busy : editSignIn ? 'Sign in' : editing ? 'Save' : 'Connect'}
                        </cds-button>`}
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

    // handleSignIn fires the event, as Sign in is clicked, so a tab can be
    // opened, as it's a click
    private handleSignIn(event: Event) {
        event.preventDefault();
        if (this.signIn) {
            this.dispatchEvent(new CustomEvent(ConnectorForm.eventSignIn, { detail: this.signIn }));
        }
    }

    // copyRedirect copies the redirect URL
    private copyRedirect() {
        if (this.redirect) {
            void navigator.clipboard?.writeText(this.redirect).catch(() => undefined);
        }
    }

    private handleSubmit(event: Event) {
        event.preventDefault();
        if (this.connecting || this.signIn || !this.item) {
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
            client_id_credential: this.clientIdInput?.value.trim() || undefined,
            client_secret_credential: this.clientSecretInput?.value.trim() || undefined,
        };
        for (const [what, name] of [['client ID', meta.client_id_credential], ['client secret', meta.client_secret_credential]]) {
            if (name && !credentialName.test(name)) {
                this.error = `Give the name of the credential which holds the ${what}, such as ${what === 'client ID' ? 'GITHUB_CLIENT_ID' : 'GITHUB_CLIENT_SECRET'}, rather than the ${what} itself`;
                return;
            }
        }
        if (meta.client_secret_credential && !meta.client_id_credential) {
            this.error = "A client secret's credential name needs a client ID's";
            return;
        }
        this.dispatchEvent(new CustomEvent(this.item.mode === 'edit' ? ConnectorForm.eventSave : ConnectorForm.eventConnect, { detail: meta }));
    }
}

// credentialName is the pattern of a credential's name, such as
// GITHUB_CLIENT_ID, as an environment variable's, as the server's is
const credentialName = /^[A-Za-z_][A-Za-z0-9_]*$/;

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
