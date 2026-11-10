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
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Upload16 from '@carbon/icons/es/upload/16.js';
import Copy16 from '@carbon/icons/es/copy/16.js';
import TrashCan16 from '@carbon/icons/es/trash-can/16.js';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/select/index.js';
import '@carbon/web-components/es/components/notification/inline-notification.js';

// AgentEditorItem is what the panel shows: a new agent, from its definition,
// or the agent with the name, its description, which is the panel's
// subtitle, its definition, and whether it's built in, when its model, which
// is all of it which can be changed, can be chosen from the models
export type AgentEditorItem = { mode: 'new'; yaml: string } | { mode: 'edit'; name: string; description?: string; yaml: string; builtin: boolean; model?: string; models?: string[] };

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

    protected renderTitle(item: AgentEditorItem) {
        return item.mode === 'new' ? 'New agent' : item.name;
    }

    protected renderSubtitle(item: AgentEditorItem) {
        if (item.mode === 'new') {
            return "Its definition, as YAML, or a file of one, which is uploaded";
        }
        return item.description?.trim() || `${item.builtin ? 'builtin' : 'agent'}:${item.name}`;
    }

    protected renderFooter(item: AgentEditorItem) {
        const builtin = item.mode === 'edit' && item.builtin;
        return html`
            ${builtin ? html`
                <cds-button kind="primary" size="md" ?disabled=${this.busy} @click=${() => this.saveModel(item.name)}>Save</cds-button>
                <cds-button kind="secondary" size="md" @click=${() => this.copy(item.name, item.yaml)}>
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
        const builtin = item.mode === 'edit' && item.builtin;
        return keyed(this.opened, html`
            ${builtin ? html`
                <cds-inline-notification kind="info" title="Built in" hide-close-button low-contrast
                    subtitle="The server creates this agent as it ships each time it starts, keeping the model chosen for it, which is all of it which can be changed, and it can't be deleted. Copy it, under another name, to change the rest."></cds-inline-notification>
                <cds-select id="model" label-text="Model" helper-text="Default is the model of the chat it's run from" value=${item.mode === 'edit' ? item.model ?? '' : ''} ?disabled=${this.busy}>
                    <cds-select-item value="">Default</cds-select-item>
                    ${(item.mode === 'edit' ? item.models ?? [] : []).map((model) => html`<cds-select-item value=${model}>${model}</cds-select-item>`)}
                </cds-select>
            ` : nothing}
            <textarea spellcheck="false" wrap="off" aria-label="Definition, as YAML" ?readonly=${builtin || this.busy} .value=${item.yaml} @focus=${this.clearError} @input=${this.clearError}></textarea>
            ${this.error ? html`<cds-inline-notification kind="error" title="It couldn't be saved" subtitle=${this.error} hide-close-button low-contrast></cds-inline-notification>` : nothing}
            <input type="file" accept=".yaml,.yml,application/yaml,text/yaml" @change=${this.upload}>
        `);
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
        this.item = { mode: 'new', yaml: copied };
    }

    // upload puts the file which is chosen in the editor, to be saved
    private async upload() {
        const file = this.file?.files?.[0];
        if (!file || !this.editor) {
            return;
        }
        this.editor.value = await file.text();
        this.file!.value = '';
        this.clearError();
    }

    // saveModel fires the event to set the built-in agent's model, which is
    // empty for the default
    private saveModel(name: string) {
        if (!this.busy) {
            this.dispatchEvent(new CustomEvent<AgentModel>(AgentEditor.eventModel, { detail: { name, model: this.modelSelect?.value ?? '' } }));
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
