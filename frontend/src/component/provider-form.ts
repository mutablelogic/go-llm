// A panel which adds a provider, from a preset, such as Ollama, or Mistral,
// which sets its type, name and URL, or edits one: its URL, the patterns of
// the names of its models, and the credential of its API key, by the name of
// an environment variable on the server, so the key itself isn't stored. It
// fires an event with what's set, and shows why it couldn't be, such as that
// the provider can't be reached.

import { css, html, nothing, type PropertyValues } from 'lit';
import { keyed } from 'lit/directives/keyed.js';
import { customElement, property, query, state } from 'lit/decorators.js';
import { Panel } from './panel';
import type { Provider, ProviderMeta } from '../schema/provider';

// Carbon components
import '@carbon/web-components/es/components/select/index.js';
import '@carbon/web-components/es/components/textarea/index.js';
import '@carbon/web-components/es/components/text-input/index.js';
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/notification/inline-notification.js';

// ProviderFormItem is what the panel shows: a provider to add, or the
// provider which is edited
export type ProviderFormItem = { mode: 'add' } | { mode: 'edit'; provider: Provider };

// Preset is a kind of provider which can be added: its type, its name, unless
// another is given, its URL, which is the type's default, when it's empty,
// whether its URL is asked for, and whether its credential is needed, or can
// be given, or isn't asked for
interface Preset {
    id: string;
    label: string;
    type: string;
    name: string;
    url: string;
    urlLabel?: string;
    credential: 'required' | 'optional' | 'none';
}

// The presets, as the command-line tool's provider-create commands are
const presets: Preset[] = [
    { id: 'ollama', label: 'Ollama', type: 'ollama', name: 'ollama', url: 'http://localhost:11434', urlLabel: 'URL of the Ollama server', credential: 'optional' },
    { id: 'openai', label: 'OpenAI', type: 'openai', name: 'openai', url: '', urlLabel: "URL, or empty for OpenAI's", credential: 'required' },
    { id: 'mistral', label: 'Mistral', type: 'openai', name: 'mistral', url: 'https://api.mistral.ai/v1', credential: 'required' },
    { id: 'deepseek', label: 'DeepSeek', type: 'openai', name: 'deepseek', url: 'https://api.deepseek.com', credential: 'required' },
    { id: 'compatible', label: 'OpenAI-compatible server, such as LM Studio or vLLM', type: 'openai', name: '', url: 'http://localhost:1234/v1', urlLabel: 'URL of the server', credential: 'optional' },
    { id: 'gowhisper', label: 'go-whisper', type: 'gowhisper', name: 'gowhisper', url: 'http://localhost:8081', urlLabel: 'URL of the go-whisper server', credential: 'none' },
    { id: 'elevenlabs', label: 'ElevenLabs', type: 'elevenlabs', name: 'elevenlabs', url: '', credential: 'required' },
    { id: 'eliza', label: 'Eliza, for testing', type: 'eliza', name: 'eliza', url: '', credential: 'none' },
];

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

    // The preset which is chosen, and how many times the panel has been
    // opened, so its form is made again each time
    @state()
    private preset = presets[0];

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

    // willUpdate starts the form again, from the first preset, or the
    // provider which is edited, each time the panel opens
    willUpdate(changed: PropertyValues<this>) {
        if (changed.has('item') && this.item && changed.get('item') === undefined) {
            this.opened++;
            this.preset = presets[0];
            this.error = undefined;
        }
        super.willUpdate(changed);
    }

    protected current(): ProviderFormItem | undefined {
        return this.item;
    }

    protected renderTitle(item: ProviderFormItem) {
        return item.mode === 'add' ? 'Add provider' : `Edit ${item.provider.name}`;
    }

    protected renderSubtitle(item: ProviderFormItem) {
        return item.mode === 'add' ? 'A provider of models, which is checked to be reachable' : `A provider of type ${item.provider.provider}`;
    }

    protected renderBody(item: ProviderFormItem) {
        const editing = item.mode === 'edit' ? item.provider : undefined;
        const preset = this.preset;
        const credential = editing ? (editing.provider === 'eliza' ? 'none' : 'optional') : preset.credential;
        const hasURL = editing ? editing.provider !== 'eliza' : preset.type !== 'eliza';
        return keyed(this.opened, html`
            <form @submit=${this.handleSubmit}>
                ${editing ? nothing : html`
                    <cds-select id="preset" label-text="Provider" value=${preset.id}
                        @cds-select-selected=${(e: CustomEvent<{ value: string }>) => { this.preset = presets.find((p) => p.id === e.detail.value) ?? presets[0]; }}>
                        ${presets.map((p) => html`<cds-select-item value=${p.id}>${p.label}</cds-select-item>`)}
                    </cds-select>
                    ${keyed(preset.id, html`
                        <cds-text-input id="name" label="Name" helper-text="Its name in the server, which its models' names start with, such as ollama:llama3.2" value=${preset.name}></cds-text-input>
                        ${hasURL ? html`<cds-text-input id="url" label=${preset.urlLabel ?? 'URL'} helper-text="Its API's URL, which is the default, when it's empty" value=${preset.url}></cds-text-input>` : nothing}
                    `)}
                `}
                ${editing && hasURL ? html`<cds-text-input id="url" label="URL" helper-text="Its API's URL, or empty, for the default" value=${editing.url ?? ''}></cds-text-input>` : nothing}
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
        const meta: ProviderMeta = {
            url: this.urlInput?.value.trim() || undefined,
            include: lines(this.includeInput?.value),
            exclude: lines(this.excludeInput?.value),
        };
        if (credential) {
            meta.api_key = '${' + credential + '}';
        }

        // A provider to add has its name, and type, which aren't changed
        if (this.item.mode === 'add') {
            if (this.preset.credential === 'required' && !credential && !(this.preset.id === 'openai' && meta.url)) {
                this.error = `${this.preset.label} needs the name of the environment variable which has its API key`;
                return;
            }
            meta.name = this.nameInput?.value.trim() || undefined;
            meta.provider = this.preset.type;
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
