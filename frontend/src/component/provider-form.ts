// A panel which adds a provider, of a preset, such as Ollama, or Mistral,
// which sets its type, name and URL, of which only a server of its own, such
// as Ollama's, is asked for, or edits one: its URL, when it has its own, the
// patterns of the names of its models, and the credential of its API key, by
// the name of an environment variable on the server, so the key itself isn't
// stored. It fires an event with what's set, and shows why it couldn't be,
// such as that the provider can't be reached.

import { css, html, nothing, type PropertyValues } from 'lit';
import { keyed } from 'lit/directives/keyed.js';
import { customElement, property, query, state } from 'lit/decorators.js';
import { Panel } from './panel';
import type { Provider, ProviderMeta } from '../schema/provider';
import { asksURL, presetOf } from '../schema/preset';

// Carbon components
import '@carbon/web-components/es/components/select/index.js';
import '@carbon/web-components/es/components/textarea/index.js';
import '@carbon/web-components/es/components/text-input/index.js';
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/notification/inline-notification.js';

// ProviderFormItem is what the panel shows: a provider to add, of a preset,
// or the provider which is edited
export type ProviderFormItem = { mode: 'add'; preset: string } | { mode: 'edit'; provider: Provider };

// credentialPattern is what the name of an environment variable is
const credentialPattern = /^[A-Za-z_][A-Za-z0-9_]*$/;

@customElement('llm-provider-form')
export class ProviderForm extends Panel<ProviderFormItem> {
    // The names of the events fired with the provider to create, and with the
    // changes to the provider which is edited
    static readonly eventCreate = 'llm-provider-form-create';
    static readonly eventSave = 'llm-provider-form-save';

    // What's shown, which is undefined when the panel is closed
    @property({ attribute: false })
    item?: ProviderFormItem;

    // Whether it's being saved, and why it couldn't be, when it couldn't
    @property({ type: Boolean })
    saving = false;

    @property({ attribute: false })
    error?: string;

    // How many times the panel has been opened, so its form is made again
    // each time
    @state()
    private opened = 0;

    @query('cds-text-input#name')
    private nameInput?: HTMLElement & { value: string };

    @query('cds-text-input#url')
    private urlInput?: HTMLElement & { value: string };

    @query('cds-text-input#credential')
    private credentialInput?: HTMLElement & { value: string };

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

    // willUpdate starts the form again, from the preset, or the provider
    // which is edited, each time what it shows changes, such as when it
    // opens, so nothing which was typed, such as a credential, is kept for
    // another provider
    willUpdate(changed: PropertyValues<this>) {
        if (changed.has('item') && this.item && changed.get('item') !== this.item) {
            this.opened++;
            this.error = undefined;
        }
        super.willUpdate(changed);
    }

    protected current(): ProviderFormItem | undefined {
        return this.item;
    }

    protected renderTitle(item: ProviderFormItem) {
        return item.mode === 'add' ? `Add ${presetOf(item.preset).label} provider` : `Edit ${item.provider.name}`;
    }

    protected renderSubtitle(item: ProviderFormItem) {
        return item.mode === 'add' ? 'A service, or a server of your own, which runs assistant models, and models which read, transcribe or embed' : `A provider of type ${item.provider.provider}`;
    }

    protected renderBody(item: ProviderFormItem) {
        const editing = item.mode === 'edit' ? item.provider : undefined;
        const preset = item.mode === 'add' ? presetOf(item.preset) : undefined;
        const credential = preset ? preset.credential : editing?.provider === 'eliza' || editing?.provider === 'gowhisper' ? 'none' : 'optional';
        const askURL = preset ? preset.askURL : asksURL(editing?.provider ?? '', editing?.url);
        return keyed(this.opened, html`
            <form @submit=${this.handleSubmit} @focusin=${this.clearError}>
                ${preset ? html`<cds-text-input id="name" label="Name" helper-text="Its name in the server, which its models' names start with, such as ollama:llama3.2" value=${preset.name}></cds-text-input>` : nothing}
                ${askURL ? html`<cds-text-input id="url" label=${preset?.urlLabel ?? 'URL'} helper-text="Its API's URL" value=${preset ? preset.url : editing?.url ?? ''}></cds-text-input>` : nothing}
                ${credential === 'none' ? nothing : html`
                    <cds-text-input id="credential" label=${credential === 'required' ? 'API key credential' : 'API key credential (optional)'}
                        placeholder=${editing ? 'Unchanged' : 'Such as MISTRAL_API_KEY'}
                        helper-text=${editing
                            ? 'The name of the environment variable on the server which has the API key, or empty, to keep the one it has'
                            : "The name of the environment variable on the server which has the API key, which is read when it's used, so the key isn't stored"}></cds-text-input>
                `}
                <cds-textarea id="include" label="Include models (optional)" rows="2" helper-text="Regular expressions of the names of the models to include, one a line, or every model" value=${(editing?.include ?? []).join('\n')}></cds-textarea>
                <cds-textarea id="exclude" label="Exclude models (optional)" rows="2" helper-text="Regular expressions of the names of the models to exclude, one a line" value=${(editing?.exclude ?? []).join('\n')}></cds-textarea>
                ${this.error ? html`<cds-inline-notification kind="error" title=${editing ? "It couldn't be saved" : "It couldn't be added"} subtitle=${this.error} hide-close-button low-contrast></cds-inline-notification>` : nothing}
                <div class="footer">
                    <cds-button kind="primary" type="submit" ?disabled=${this.saving} @click=${this.handleSubmit}>${editing ? 'Save' : 'Add provider'}</cds-button>
                </div>
            </form>
        `);
    }

    // clearError clears why it couldn't be added, or saved, once a field is
    // focused, so it's changed, apart from the button, which is focused as
    // it's clicked, so it's tried again
    private clearError(event: FocusEvent) {
        if (this.error && !(event.target instanceof Element && event.target.closest('cds-button'))) {
            this.error = undefined;
        }
    }

    private handleSubmit(event: Event) {
        event.preventDefault();
        if (this.saving || !this.item) {
            return;
        }

        // The credential, which is the name of an environment variable, sent
        // as its placeholder, such as ${MISTRAL_API_KEY}
        const credential = this.credentialInput?.value.trim() ?? '';
        if (credential && !credentialPattern.test(credential)) {
            this.error = `${credential} isn't the name of an environment variable, such as MISTRAL_API_KEY`;
            return;
        }
        // The URL which is asked for, or, when it isn't, the preset's, or the
        // provider's, as it is, as an update without one clears it
        const item = this.item;
        const url = this.urlInput
            ? this.urlInput.value.trim()
            : item.mode === 'add' ? presetOf(item.preset).url : item.provider.url ?? '';
        const meta: ProviderMeta = {
            url: url || undefined,
            include: lines(this.includeInput?.value),
            exclude: lines(this.excludeInput?.value),
        };
        if (credential) {
            meta.api_key = '${' + credential + '}';
        }

        // A provider to add has its name, and type, which aren't changed
        if (item.mode === 'add') {
            const preset = presetOf(item.preset);
            if (preset.credential === 'required' && !credential) {
                this.error = `${preset.label} needs the name of the environment variable which has its API key`;
                return;
            }
            meta.name = this.nameInput?.value.trim() || undefined;
            meta.provider = preset.type;
            this.dispatchEvent(new CustomEvent(ProviderForm.eventCreate, { detail: meta }));
            return;
        }
        this.dispatchEvent(new CustomEvent(ProviderForm.eventSave, { detail: meta }));
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
        'llm-provider-form': ProviderForm;
    }
}
