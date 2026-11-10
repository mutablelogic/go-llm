// A panel which loads a model with a provider which stores models, such as
// Ollama, which downloads it when it isn't stored, by its name, and shows how
// the download is getting on, and why it couldn't be loaded, when it
// couldn't. It fires an event with the model's name.

import { css, html, nothing, type PropertyValues } from 'lit';
import { keyed } from 'lit/directives/keyed.js';
import { customElement, property, query, state } from 'lit/decorators.js';
import { Panel } from './panel';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Download16 from '@carbon/icons/es/download/16.js';
import type { LoadProgress } from '../api/model';

// Carbon components
import '@carbon/web-components/es/components/text-input/index.js';
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/progress-bar/index.js';
import '@carbon/web-components/es/components/notification/inline-notification.js';

// examples are a model's name, for each type of provider, as a hint
const examples: Record<string, string> = {
    ollama: 'Such as llama3.2:latest, or qwen3:8b, from ollama.com/library',
    gowhisper: 'Such as ggml-medium-q5_0, or ggml-large-v3-turbo',
};

@customElement('llm-model-load')
export class ModelLoad extends Panel<string> {
    // The name of the event fired with the name of the model to load
    static readonly eventLoad = 'llm-model-load';

    // The provider which loads it, which is undefined when the panel is
    // closed, and its type, for a hint of a model's name
    @property({ attribute: false })
    provider?: string;

    @property({ attribute: false })
    type = '';

    // Whether it's loading, how it's getting on, and why it couldn't be
    // loaded, when it couldn't
    @property({ type: Boolean })
    loading = false;

    @property({ attribute: false })
    progress?: LoadProgress;

    @property({ attribute: false })
    error?: string;

    // How many times the panel has been opened, so its form is made again
    // each time
    @state()
    private opened = 0;

    @query('cds-text-input#name')
    private nameInput?: HTMLElement & { value: string };

    static styles = [Panel.styles, css`
        form > * {
            display: block;
            margin-bottom: 1.5rem;
        }
    `];

    willUpdate(changed: PropertyValues<this>) {
        if (changed.has('provider') && this.provider && changed.get('provider') === undefined) {
            this.opened++;
            this.error = undefined;
            this.progress = undefined;
        }
        super.willUpdate(changed);
    }

    protected current() {
        return this.provider;
    }

    protected renderIcon(provider: string) {
        return iconLoader(Download16);
    }

    protected renderTitle(provider: string) {
        return `Load a model with ${provider}`;
    }

    protected renderSubtitle() {
        return "It's downloaded, when it isn't stored already";
    }

    protected renderFooter() {
        return html`<cds-button kind="primary" type="submit" ?disabled=${this.loading} @click=${this.handleSubmit}>Load model</cds-button>`;
    }

    protected renderBody() {
        const percent = this.progress?.percent;
        return keyed(this.opened, html`
            <form @submit=${this.handleSubmit} @focusin=${this.clearError}>
                <cds-text-input id="name" label="Model" helper-text=${examples[this.type] ?? 'The name of the model'} ?disabled=${this.loading}></cds-text-input>
                ${this.loading ? html`
                    <cds-progress-bar label=${this.progress?.status ?? 'Starting'}
                        value=${percent === undefined ? nothing : Math.round(percent)} max="100"
                        helper-text=${percent === undefined ? '' : `${Math.round(percent)}%`}></cds-progress-bar>
                ` : nothing}
                ${this.error ? html`<cds-inline-notification kind="error" title="It couldn't be loaded" subtitle=${this.error} hide-close-button low-contrast></cds-inline-notification>` : nothing}
            </form>
        `);
    }

    // clearError clears why it couldn't be loaded, once the name is focused
    private clearError(event: FocusEvent) {
        if (this.error && !(event.target instanceof Element && event.target.closest('cds-button'))) {
            this.error = undefined;
        }
    }

    private handleSubmit(event: Event) {
        event.preventDefault();
        const name = this.nameInput?.value.trim() ?? '';
        if (this.loading || !name) {
            return;
        }
        this.dispatchEvent(new CustomEvent(ModelLoad.eventLoad, { detail: name }));
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-model-load': ModelLoad;
    }
}
