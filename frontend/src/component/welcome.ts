// The welcome, which is shown in place of a chat when there are none yet:
// the providers, and how many models each has, with a button to add one,
// a button to start a chat, which chooses its model, and what to try, of
// what the models can do, such as speak, or make images

import { LitElement, css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { cannot, type Model } from '../schema/model';
import type { Provider } from '../schema/provider';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/skeleton-text/index.js';

@customElement('llm-welcome')
export class Welcome extends LitElement {
    // The names of the events fired to start a chat, and to add a provider
    static readonly eventNew = 'llm-welcome-new';
    static readonly eventProvider = 'llm-welcome-provider';

    // The providers, and the models, which are undefined while they load
    @property({ attribute: false })
    providers?: Provider[];

    @property({ attribute: false })
    models?: Model[];

    static styles = css`
        :host {
            display: block;
            max-width: 40rem;
            margin: auto;
            padding: 2rem;
            color: var(--cds-text-primary);
        }

        h2 {
            margin: 0 0 0.5rem;
            font-size: 1.75rem;
            font-weight: 400;
            line-height: 2.25rem;
        }

        p {
            margin: 0 0 1.5rem;
            color: var(--cds-text-secondary);
            line-height: 1.5;
        }

        h3 {
            margin: 2rem 0 0.75rem;
            font-size: 0.875rem;
            font-weight: 600;
            letter-spacing: 0.16px;
        }

        ul {
            margin: 0;
            padding: 0;
            list-style: none;
        }

        li {
            padding: 0.5rem 0;
            border-bottom: 1px solid var(--cds-border-subtle);
        }

        .provider {
            display: flex;
            justify-content: space-between;
            gap: 1rem;
        }

        .count, .note {
            color: var(--cds-text-secondary);
        }

        .error {
            color: var(--cds-text-error, #da1e28);
        }

        .actions {
            display: flex;
            flex-wrap: wrap;
            gap: 1rem;
            margin-top: 1rem;
        }

        ul.try {
            padding-left: 1.25rem;
            list-style: disc;
        }

        .try li {
            border: none;
            padding: 0.25rem 0;
            line-height: 1.5;
        }

        .try li::marker {
            color: var(--cds-text-secondary);
        }
    `;

    render() {
        const { providers, models } = this;
        if (!providers || !models) {
            return html`<cds-skeleton-text paragraph lines="4"></cds-skeleton-text>`;
        }
        const real = providers.filter((provider) => provider.provider !== 'eliza');
        return html`
            <h2>Welcome</h2>
            <p>${real.length > 0
                ? 'Start a conversation with your assistant, and see what they can do.'
                : providers.length > 0
                    ? 'Add a provider, such as Ollama, OpenAI, Anthropic or Gemini, to chat with its models. Until then, there\'s only Eliza, a mock of the 1966 chatbot, for testing.'
                    : 'Add a provider, such as Ollama, OpenAI, Anthropic or Gemini, to chat with its models.'}</p>

            <h3>Providers</h3>
            <ul>
                ${providers.length === 0 ? html`<li class="note">None yet</li>` : nothing}
                ${providers.map((provider) => html`
                    <li class="provider">
                        <span>${provider.name}${provider.provider === 'eliza' ? html` <span class="note">(a mock, for testing)</span>` : nothing}</span>
                        ${provider.error
                            ? html`<span class="error">can't be reached</span>`
                            : html`<span class="count">${plural(count(models, provider.name), 'model', 'models')}</span>`}
                    </li>
                `)}
            </ul>

            <div class="actions">
                ${models.some((model) => !cannot(model, 'chat')) ? html`
                    <cds-button kind="primary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(Welcome.eventNew, { bubbles: true, composed: true }))}>
                        Start a chat
                    </cds-button>
                ` : nothing}
                <cds-button kind=${real.length > 0 ? 'tertiary' : 'primary'} size="md" @click=${() => this.dispatchEvent(new CustomEvent(Welcome.eventProvider, { bubbles: true, composed: true }))}>
                    Add a provider
                </cds-button>
            </div>

            ${this.renderTry(models)}
        `;
    }

    // renderTry returns what to try, of what the models can do, or nothing
    private renderTry(models: Model[]) {
        const can = (capability: Parameters<typeof cannot>[1]) => models.some((model) => model.capabilities?.[capability] === true);
        const tries = [
            'Attach a file, by dropping it onto the conversation, or pasting it, and ask about it',
            "Choose the tools a conversation can use, such as the weather, the news, or the assistant's long-term memory, when you start it",
            can('tools') ? "Set a reminder, such as 'remind me to call Sam at five'" : '',
            can('image_output') ? 'Ask for a picture, or to change one' : '',
            can('speech_output') ? 'Ask for something to be read aloud' : '',
            can('transcription') ? 'Attach a recording, and ask what\'s said in it' : '',
            can('ocr') || can('document_input') ? 'Attach a scan, or a PDF, and ask what it says' : '',
        ].filter((text) => text !== '');
        return html`
            <h3>Try</h3>
            <ul class="try">${tries.map((text) => html`<li>${text}</li>`)}</ul>
        `;
    }
}

// count returns how many of the models are the provider's
function count(models: Model[], provider: string): number {
    return models.filter((model) => model.provider === provider).length;
}

// plural returns how many, with the singular, for one, or the plural
function plural(n: number, singular: string, plural: string): string {
    return `${n} ${n === 1 ? singular : plural}`;
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-welcome': Welcome;
    }
}
