// A panel which starts a new chat, without a title, which it's given, and
// which compacting it changes, or edits one, with its title: its model, of those which aren't known
// not to generate text, its system prompt, its options, which are the
// model's own unless they're set, such as whether it thinks, and the tools
// it can call, unless it's known not to call them. The button which starts
// it stays at the bottom of the panel. It fires an event with the new
// session's settings, when it's created.

import { css, html, type PropertyValues, nothing } from 'lit';
import { keyed } from 'lit/directives/keyed.js';
import { customElement, property, query, state } from 'lit/decorators.js';
import { Panel } from './panel';
import { cannot, type Model } from '../schema/model';
import type { Namespace, Tool } from '../schema/tool';
import type { ToolPicker } from './tool-picker';
import './tool-picker';
import type { Session, SessionMeta } from '../schema/session';

// Carbon components
import '@carbon/web-components/es/components/select/index.js';
import '@carbon/web-components/es/components/textarea/index.js';
import '@carbon/web-components/es/components/text-input/index.js';
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/skeleton-text/index.js';
import './disclosure';

// NewSessionForm is what the panel shows, which is always so, while it's open
type NewSessionForm = true;

@customElement('llm-new-session')
export class NewSession extends Panel<NewSessionForm> {
    // The names of the events fired with the new session's settings, as its
    // detail, and with the changes to the session which is edited
    static readonly eventCreate = 'llm-new-session-create';
    static readonly eventSave = 'llm-new-session-save';

    // The session which is edited, or none, for a new chat
    @property({ attribute: false })
    session?: Session;

    // How many times the panel has been opened, so its form is made again
    // each time, with the settings it starts with, and the tools it starts
    // with, which are the session's, or every tool, for a new chat
    @state()
    private opened = 0;

    private startTools: string[] = ['*'];

    // Whether the panel is shown
    @property({ type: Boolean })
    visible = false;

    // The models which can be chosen, which are undefined while they're
    // loading, and the model which is chosen first
    @property({ attribute: false })
    models?: Model[];

    @property({ attribute: false })
    model?: string;

    // The canonical name of the model which is chosen, as it's changed
    @property({ attribute: false })
    chosen?: string;

    // The tools which can be chosen, which are kept current, and are
    // undefined while they're loading
    @property({ attribute: false })
    tools?: Tool[];

    // The namespaces of the tools, with what they're for
    @property({ attribute: false })
    namespaces: Namespace[] = [];

    // Whether the session is being created
    @property({ type: Boolean })
    creating = false;

    // Whether the model thinks, which is its own default when it's default, so
    // its effort isn't asked for when it doesn't
    @property({ attribute: false })
    thinking = 'default';

    @query('cds-select#model')
    private select?: HTMLElement & { value: string };

    @query('cds-select#thinking')
    private thinkingSelect?: HTMLElement & { value: string };

    @query('cds-select#effort')
    private effort?: HTMLElement & { value: string };

    @query('cds-text-input#temperature')
    private temperature?: HTMLElement & { value: string };

    @query('cds-textarea')
    private system?: HTMLElement & { value: string };

    @query('cds-text-input#title')
    private titleInput?: HTMLElement & { value: string };

    @query('llm-tool-picker')
    private picker?: ToolPicker;

    static styles = [Panel.styles, css`
        form > * {
            display: block;
            margin-bottom: 1.5rem;
        }

        /* The advanced options, and the tools, are spaced as the form is */
        llm-disclosure > * {
            display: block;
            margin-bottom: 1.5rem;
        }

        .note {
            color: var(--cds-text-secondary);
            font-size: 0.875rem;
            line-height: 1.43;
        }

        /* The options are side by side, when there's room */
        .options {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(8rem, 1fr));
            gap: 1rem;
        }

    `];

    // willUpdate starts the form again, from the session's settings, or the
    // defaults, each time the panel opens
    willUpdate(changed: PropertyValues<this>) {
        if (changed.has('visible') && this.visible) {
            const opt = this.session?.opt ?? {};
            this.opened++;
            this.chosen = this.session?.model;
            this.thinking = opt.thinking === true ? 'true' : opt.thinking === false ? 'false' : 'default';
            this.startTools = this.session ? [...(this.session.tools ?? [])] : ['*'];
        }
        super.willUpdate(changed);
    }

    protected current(): NewSessionForm | undefined {
        return this.visible ? true : undefined;
    }

    protected renderTitle() {
        return this.session ? 'Edit chat' : 'New chat';
    }

    protected renderSubtitle() {
        return this.session ? 'Its title, model, instructions, options and tools' : 'A conversation with your assistant';
    }

    protected renderFooter() {
        return this.models
            ? html`<cds-button kind="primary" type="submit" ?disabled=${this.creating} @click=${this.handleSubmit}>${this.session ? 'Save' : 'Start chat'}</cds-button>`
            : nothing;
    }

    protected renderBody() {
        if (!this.models) {
            return html`<cds-skeleton-text paragraph line-count="3"></cds-skeleton-text>`;
        } else if (this.models.length === 0) {
            return html`<p>There are no models which generate text. Add a provider, such as Ollama, or OpenAI, first.</p>`;
        }
        const chosen = [this.chosen, this.model].find((name) => this.models?.some((model) => model.canonical === name)) ?? this.models[0].canonical;
        const model = this.models.find((model) => model.canonical === chosen);
        const thinks = !cannot(model, 'thinking');
        const opt = this.session?.opt ?? {};
        return keyed(this.opened, html`
            <form @submit=${this.handleSubmit}>
                ${this.session ? html`
                    <cds-text-input id="title" label="Title" helper-text="What the chat is about, which it's given each time it's compacted"
                        value=${this.session.title ?? ''}></cds-text-input>
                ` : nothing}
                <cds-select id="model" label-text="Assistant model" value=${chosen}
                    @cds-select-selected=${(e: CustomEvent<{ value: string }>) => { this.chosen = e.detail.value; }}>
                    ${this.models.map((model) => html`<cds-select-item value=${model.canonical}>${model.canonical}</cds-select-item>`)}
                </cds-select>
                <cds-textarea label="System prompt" helper-text="Instructions for the assistant on how to act, such as 'be brief', 'set humour level to 90%', or 'respond in rhyme'" rows="3"
                    value=${this.session?.system ?? ''}></cds-textarea>
                <llm-disclosure label="Advanced options">
                <div class="options">
                    ${thinks ? html`
                        <cds-select id="thinking" label-text="Thinking" value=${this.thinking}
                            @cds-select-selected=${(e: CustomEvent<{ value: string }>) => { this.thinking = e.detail.value; }}>
                            <cds-select-item value="default">Default</cds-select-item>
                            <cds-select-item value="true">On</cds-select-item>
                            <cds-select-item value="false">Off</cds-select-item>
                        </cds-select>
                        ${this.thinking !== 'false' ? html`
                            <cds-select id="effort" label-text="Effort" value=${typeof opt.thinking_effort === 'string' ? opt.thinking_effort : 'default'}>
                                <cds-select-item value="default">Default</cds-select-item>
                                <cds-select-item value="low">Low</cds-select-item>
                                <cds-select-item value="medium">Medium</cds-select-item>
                                <cds-select-item value="high">High</cds-select-item>
                                <cds-select-item value="xhigh">Extra high</cds-select-item>
                                <cds-select-item value="max">Max</cds-select-item>
                            </cds-select>
                        ` : ''}
                    ` : ''}
                    ${cannot(model, 'temperature') ? '' : html`
                        <cds-text-input id="temperature" label="Temperature" type="number" min="0" max="2" step="0.1" placeholder="Default"
                            value=${typeof opt.temperature === 'number' ? String(opt.temperature) : ''}></cds-text-input>
                    `}
                </div>
                ${cannot(model, 'tools')
                    ? ''
                    : html`
                        <p class="note">Every tool is on, unless you turn it off. Without the <strong>llm</strong> tools, the assistant can't speak, make pictures, read scans or transcribe recordings, or read the chat's files; without <strong>memory</strong>, it doesn't remember you from one chat to the next; and without <strong>heartbeat</strong>, it can't set reminders, or do things on a schedule.</p>
                        <llm-tool-picker .tools=${this.tools} .described=${this.namespaces} .value=${this.startTools}></llm-tool-picker>
                    `}
                </llm-disclosure>
            </form>
        `);
    }

    private handleSubmit(event: Event) {
        event.preventDefault();
        if (this.creating || !this.select?.value) {
            return;
        }
        const model = this.models?.find((model) => model.canonical === this.select?.value);
        const title = this.titleInput?.value.trim() ?? '';
        const system = this.system?.value.trim() ?? '';
        const tools = cannot(model, 'tools') ? [] : this.picker?.value ?? [];

        // The options which are set, rather than the model's own
        const opt: Record<string, unknown> = {};
        if (this.thinkingSelect?.value && this.thinkingSelect.value !== 'default') {
            opt.thinking = this.thinkingSelect.value === 'true';
        }
        if (this.effort?.value && this.effort.value !== 'default') {
            opt.thinking_effort = this.effort.value;
        }
        const temperature = parseFloat(this.temperature?.value ?? '');
        if (Number.isFinite(temperature)) {
            opt.temperature = temperature;
        }

        // A new chat has what's set, and the session which is edited what's
        // changed: an empty title, or system prompt, and no tools, clear them, and an
        // option which is the model's own again is removed, with null
        const session = this.session;
        if (!session) {
            const meta: SessionMeta = { model: model?.canonical };
            if (title) {
                meta.title = title;
            }
            if (system) {
                meta.system = system;
            }
            if (Object.keys(opt).length > 0) {
                meta.opt = opt;
            }
            if (tools.length > 0) {
                meta.tools = tools;
            }
            this.dispatchEvent(new CustomEvent(NewSession.eventCreate, { detail: meta }));
            return;
        }
        const changes: SessionMeta = {};
        if (title !== (session.title ?? '')) {
            changes.title = title;
        }
        if (model && model.canonical !== session.model) {
            changes.model = model.canonical;
        }
        if (system !== (session.system ?? '')) {
            changes.system = system;
        }
        if ([...tools].sort().join() !== [...(session.tools ?? [])].sort().join()) {
            changes.tools = tools;
        }
        const changed: Record<string, unknown> = {};
        for (const key of ['thinking', 'thinking_effort', 'temperature']) {
            const before = session.opt?.[key];
            if (opt[key] !== before) {
                changed[key] = key in opt ? opt[key] : null;
            }
        }
        if (Object.keys(changed).length > 0) {
            changes.opt = changed;
        }
        this.dispatchEvent(new CustomEvent(NewSession.eventSave, { detail: changes }));
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-new-session': NewSession;
    }
}
