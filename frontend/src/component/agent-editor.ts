// A panel which edits an agent's definition, as YAML, as an agent's file is
// written, though the server returns it without its comments: a new agent, from a template, or a file which is uploaded, or an
// agent, which is saved, replacing it, or deleted. A built-in agent, which
// the server creates as it ships each time it starts, is shown, but can't be
// changed, or deleted, and can be copied, under another name. It fires
// events to save, and delete, and shows why it couldn't, when it couldn't.

import { css, html, nothing, type PropertyValues } from 'lit';
import { keyed } from 'lit/directives/keyed.js';
import { customElement, property, query, state } from 'lit/decorators.js';
import { Panel } from './panel';
import { namespaceIcon } from './namespace-icon';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Upload16 from '@carbon/icons/es/upload/16.js';
import Copy16 from '@carbon/icons/es/copy/16.js';
import TrashCan16 from '@carbon/icons/es/trash-can/16.js';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/select/index.js';
import '@carbon/web-components/es/components/notification/inline-notification.js';

// AgentEditorItem is what the panel shows: a new agent, from its definition,
// or the agent with the name, its title and description, which are the
// panel's title and subtitle, its definition, and whether it's built in, when its model, which
// is all of it which can be changed, can be chosen from the models
export type AgentEditorItem = { mode: 'new'; yaml: string; models?: string[] } | { mode: 'edit'; name: string; title?: string; description?: string; yaml: string; builtin: boolean; origin?: string; model?: string; models?: string[] };

// AgentModel is the model chosen for a built-in agent, by its name, which
// is empty for the model of the session it's run from
export interface AgentModel {
    name: string;
    model: string;
}

// agentTemplate is the definition a new agent starts from
export const agentTemplate = `# An agent, which models call as a tool, in the agent namespace, as
# agent:<name>, with its input, which its prompt is executed with
name: my_agent
title: My agent
description: |
  What the agent does, for people, and for the models which call it.

# Its model, or none, for the model of the session it's run from
# model: ollama:llama3.2:latest

system: |
  You are ...

# The input, as a JSON schema of an object, whose properties the prompt uses
input:
  type: object
  properties:
    text:
      type: string
      description: The text
  required:
    - text

# The message to the model, as a Go template of the input
prompt: |
  {{ .text }}
`;

// defaultModel is the select's value for the default model, rather than
// empty, which the select takes as nothing chosen, so it shows nothing
const defaultModel = 'default';

// modelValue returns the model which the select's value is, which is empty
// for the default
function modelValue(value: string | undefined): string {
    return !value || value === defaultModel ? '' : value;
}

// modelLine is a definition's model, at the top level, rather than in a
// comment, or nested
const modelLine = /^model:[ \t]*(.*?)[ \t]*$/m;

// modelOf returns the model of the definition, as YAML, or undefined, when
// it has none, without quotes
export function modelOf(yaml: string): string | undefined {
    const value = modelLine.exec(yaml)?.[1]?.replace(/\s+#.*$/, '').replace(/^(['"])(.*)\1$/, '$2').trim();
    return value || undefined;
}

// withModel returns the definition, as YAML, with the model, which replaces
// its model, or follows its name, when it has none, or without its model,
// when it's empty, for the default
export function withModel(yaml: string, model: string): string {
    if (!model) {
        return yaml.replace(/^model:.*(\r?\n|$)/m, '');
    } else if (modelLine.test(yaml)) {
        return yaml.replace(modelLine, `model: ${model}`);
    }
    const name = /^name:.*$/m.exec(yaml);
    if (!name) {
        return `model: ${model}\n${yaml}`;
    }
    const end = name.index + name[0].length;
    return `${yaml.slice(0, end)}\nmodel: ${model}${yaml.slice(end)}`;
}

@customElement('llm-agent-editor')
export class AgentEditor extends Panel<AgentEditorItem> {
    // The names of the events fired to save the definition, which is the
    // detail, and to delete the agent, whose name is the detail
    static readonly eventSave = 'llm-agent-editor-save';
    static readonly eventDelete = 'llm-agent-editor-delete';
    static readonly eventModel = 'llm-agent-editor-model';

    // What's shown, which is undefined when the panel is closed
    @property({ attribute: false })
    item?: AgentEditorItem;

    // Whether it's saving, or deleting, when the definition can't be changed,
    // so nothing typed then is lost, and why it couldn't, when it couldn't
    @property({ type: Boolean })
    busy = false;

    @property({ attribute: false })
    error?: string;

    // How many times what's shown has changed, so the editor is made again
    @state()
    private opened = 0;

    @query('textarea')
    private editor?: HTMLTextAreaElement;

    @query('input[type=file]')
    private file?: HTMLInputElement;

    @query('cds-select#model')
    private modelSelect?: HTMLElement & { value: string };

    static styles = [Panel.styles, css`
        /* The panel is wider than others, for the definition */
        .panel {
            width: min(44rem, 100vw);
        }

        textarea {
            display: block;
            box-sizing: border-box;
            width: 100%;
            min-height: 28rem;
            margin-bottom: 1rem;
            padding: 0.75rem;
            border: none;
            border-bottom: 1px solid var(--cds-border-strong-01);
            background: var(--cds-field-01);
            color: var(--cds-text-primary);
            font-family: var(--cds-code-01-font-family, 'IBM Plex Mono', monospace);
            font-size: 0.8125rem;
            line-height: 1.5;
            resize: vertical;
            tab-size: 2;
        }

        textarea:focus {
            outline: 2px solid var(--cds-focus);
            outline-offset: -2px;
        }

        textarea[readonly] {
            color: var(--cds-text-secondary);
        }


        cds-inline-notification {
            margin-bottom: 1rem;
            max-width: none;
        }

        input[type=file] {
            display: none;
        }
    `];

    willUpdate(changed: PropertyValues<this>) {
        if (changed.has('item') && this.item && changed.get('item') !== this.item) {
            this.opened++;
            this.error = undefined;
        }
        super.willUpdate(changed);
    }

    protected current() {
        return this.item;
    }

    protected renderIcon(item: AgentEditorItem) {
        return namespaceIcon('agent');
    }

    protected renderTitle(item: AgentEditorItem) {
        return item.mode === 'new' ? 'New agent' : item.title?.trim() || item.name;
    }

    protected renderSubtitle(item: AgentEditorItem) {
        if (item.mode === 'new') {
            return "Its definition, as YAML, or a file of one, which is uploaded";
        }
        return item.description?.trim() || `${item.origin ?? (item.builtin ? 'builtin' : 'agent')}:${item.name}`;
    }

    protected renderFooter(item: AgentEditorItem) {
        const builtin = item.mode === 'edit' && item.builtin;
        if (item.mode === 'edit' && item.origin) {
            // An agent which comes from elsewhere, such as a connector's
            // prompt, whose text is its server's, is only shown
            return nothing;
        }
        return html`
            ${builtin ? html`
                <cds-button kind="primary" size="md" ?disabled=${this.busy} @click=${() => this.saveModel(item.name)}>Save</cds-button>
                <cds-button kind="secondary" size="md" ?disabled=${this.busy} @click=${() => this.copy(item.name, this.editor?.value ?? item.yaml)}>
                    Copy ${iconLoader(Copy16, { slot: 'icon' })}
                </cds-button>
            ` : html`
                <cds-button kind="primary" size="md" ?disabled=${this.busy} @click=${this.save}>Save</cds-button>
                ${item.mode === 'new' ? html`
                    <cds-button kind="secondary" size="md" ?disabled=${this.busy} @click=${() => this.file?.click()}>
                        Upload YAML ${iconLoader(Upload16, { slot: 'icon' })}
                    </cds-button>
                ` : html`
                    <cds-button kind="danger" size="md" ?disabled=${this.busy}
                        @click=${() => this.dispatchEvent(new CustomEvent(AgentEditor.eventDelete, { detail: item.name }))}>
                        Delete ${iconLoader(TrashCan16, { slot: 'icon' })}
                    </cds-button>
                `}
            `}
        `;
    }

    protected renderBody(item: AgentEditorItem) {
        const origin = item.mode === 'edit' ? item.origin : undefined;
        if (origin) {
            return keyed(this.opened, html`
                <cds-inline-notification kind="info" title="From ${origin}" hide-close-button low-contrast
                    subtitle="This agent is the ${origin} connector's prompt, whose text its server has, so it runs with the model of the conversation it's run from, and can't be changed, or deleted."></cds-inline-notification>
                <textarea spellcheck="false" wrap="off" aria-label="Definition, as YAML" readonly .value=${item.yaml}></textarea>
            `);
        }
        const builtin = item.mode === 'edit' && item.builtin;
        return keyed(this.opened, html`
            ${builtin ? html`
                <cds-inline-notification kind="info" title="Built in" hide-close-button low-contrast
                    subtitle="The server creates this agent as it ships each time it starts, keeping the model chosen for it, which is all of it which can be changed, and it can't be deleted. Copy it, under another name, to change the rest."></cds-inline-notification>
            ` : nothing}
            ${this.renderModel(item)}
            <textarea spellcheck="false" wrap="off" aria-label="Definition, as YAML" ?readonly=${builtin || this.busy} .value=${item.yaml} @focus=${this.clearError} @input=${this.edited}></textarea>
            ${this.error ? html`<cds-inline-notification kind="error" title="It couldn't be saved" subtitle=${this.error} hide-close-button low-contrast></cds-inline-notification>` : nothing}
            <input type="file" accept=".yaml,.yml,application/yaml,text/yaml" @change=${this.upload}>
        `);
    }

    // renderModel returns the model, which is chosen from the models, or the
    // default, which sets the model in the definition, which, when it's
    // edited, sets the model, so they're the same. A model which isn't one
    // of the models, such as one which was typed, is shown as it is.
    private renderModel(item: AgentEditorItem) {
        const models = item.models ?? [];
        const chosen = modelOf(item.yaml);
        const options = chosen && !models.includes(chosen) ? [chosen, ...models] : models;
        return html`
            <cds-select id="model" label-text="Model" helper-text="Default is the model of the conversation it's run from" value=${chosen ?? defaultModel} ?disabled=${this.busy}
                @cds-select-selected=${(e: CustomEvent<{ value: string }>) => this.chooseModel(modelValue(e.detail.value))}>
                <cds-select-item value=${defaultModel}>Default</cds-select-item>
                ${options.map((model) => html`<cds-select-item value=${model}>${model}</cds-select-item>`)}
            </cds-select>
        `;
    }

    // chooseModel sets the model in the definition, or removes it, for the
    // default
    private chooseModel(model: string) {
        if (this.editor) {
            this.editor.value = withModel(this.editor.value, model);
        }
        this.clearError();
    }

    // edited clears why it couldn't be saved, and sets the model, as the
    // definition says, once it's changed
    private edited() {
        this.clearError();
        if (this.modelSelect && this.editor) {
            this.modelSelect.value = modelOf(this.editor.value) ?? defaultModel;
        }
    }

    // clearError clears why it couldn't be saved, once the definition is
    // focused, or changed
    private clearError() {
        if (this.error) {
            this.error = undefined;
        }
    }

    // copy shows a new agent, whose definition is the built-in agent's,
    // under another name
    private copy(name: string, yaml: string) {
        const copied = yaml.replace(/^name:\s*.*$/m, `name: ${name}_copy`);
        this.item = { mode: 'new', yaml: copied, models: this.item?.models };
    }

    // upload puts the file which is chosen in the editor, to be saved
    private async upload() {
        const file = this.file?.files?.[0];
        if (!file || !this.editor) {
            return;
        }
        this.editor.value = await file.text();
        this.file!.value = '';
        this.edited();
    }

    // saveModel fires the event to set the built-in agent's model, which is
    // empty for the default
    private saveModel(name: string) {
        if (!this.busy) {
            this.dispatchEvent(new CustomEvent<AgentModel>(AgentEditor.eventModel, { detail: { name, model: modelValue(this.modelSelect?.value) } }));
        }
    }

    private save() {
        const yaml = this.editor?.value ?? '';
        if (!this.busy && yaml.trim() !== '') {
            this.dispatchEvent(new CustomEvent(AgentEditor.eventSave, { detail: yaml }));
        }
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-agent-editor': AgentEditor;
    }
}
