// A panel which starts a new chat, without a title, which it's given, and
// which compacting it changes, or edits one, with its title: its model, of those which aren't known
// not to generate text, its system prompt, its options, which are the
// model's own unless they're set, such as whether it thinks, and the tools
// it can call, unless it's known not to call them. The button which starts
// it stays at the bottom of the panel. It fires an event with the new
// session's settings, when it's created.

import { css, html, type PropertyValues, nothing } from 'lit';
import { keyed } from 'lit/directives/keyed.js';
import { live } from 'lit/directives/live.js';
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
import '@carbon/web-components/es/components/checkbox/index.js';
import '@carbon/web-components/es/components/skeleton-text/index.js';
import '@carbon/web-components/es/components/slider/index.js';
import './disclosure';

// The context windows which can be chosen, in tokens, from 16K, as fewer is
// too few for an assistant with tools, to the model's limit, when it's
// known, or 2M, which is the model's own. The slider is the logarithm of the
// window, so each doubling is as far as the last.
const minWindow = 16 * 1024;
const maxWindow = 2 * 1024 * 1024;

// windowTop returns the most the slider has, the model's limit, when it's
// known, at which the model's own is chosen
function windowTop(limit?: number): number {
    return limit ?? maxWindow;
}

// windowOf returns the window at the slider's position, in whole thousands
// of tokens, 1024 each, so it's 16K, 37K, or 128K
function windowOf(position: number): number {
    return Math.max(1, Math.round(2 ** position / 1024)) * 1024;
}

// windowPosition returns the slider's position for the window, which is the
// top, for the model's own, or one as large as its limit
function windowPosition(window: number | undefined, limit?: number): number {
    const top = windowTop(limit);
    if (!window || window >= top) {
        return Math.log2(top);
    }
    return Math.max(Math.log2(Math.min(minWindow, top)), Math.log2(window));
}

// tokens returns a number of tokens, such as 128K, or 1M, in powers of two
// when it's one, as the windows are, or otherwise thousands, as a model's
// limit, such as 200,000, may be
function tokens(n: number): string {
    if (n >= 1024 * 1024 && n % (1024 * 1024) === 0) {
        return `${n / (1024 * 1024)}M`;
    } else if (n % 1024 === 0) {
        return `${n / 1024}K`;
    } else if (n >= 1000000) {
        return `${Number((n / 1000000).toFixed(1))}M`;
    }
    return `${Math.round(n / 1000)}K`;
}


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

    // The context window the session asks for, in tokens, or undefined, for
    // the model's own, which is kept as it is, whatever its model, as the
    // registry asks a model with no more than it can have
    @state()
    private contextWindow?: number;

    @query('cds-select#model')
    private select?: HTMLElement & { value: string };

    @query('cds-select#thinking')
    private thinkingSelect?: HTMLElement & { value: string };

    @query('cds-select#effort')
    private effort?: HTMLElement & { value: string };

    @query('cds-checkbox#trace')
    private traceBox?: HTMLElement & { checked: boolean };

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
            margin-bottom: 2rem;
        }

        /* The advanced options, and the tools, are spaced as the form is */
        llm-disclosure > * {
            display: block;
            margin-bottom: 2rem;
        }

        llm-disclosure > *:first-child {
            margin-top: 1rem;
        }

        /* Each control's label stands out from what it says about itself,
           in the text's colour, and larger, which Carbon's labels take from
           these */
        cds-select, cds-text-input, cds-textarea, cds-slider, llm-tool-picker {
            --cds-text-secondary: var(--cds-text-primary);
            --cds-label-01-font-size: 0.875rem;
            --cds-label-01-letter-spacing: 0.16px;
        }

        /* A note is helper text, as Carbon's controls have */
        .note {
            margin: 0.25rem 0 0;
            color: var(--cds-text-helper);
            font-size: 0.75rem;
            line-height: 1.33;
            letter-spacing: 0.32px;
        }

        /* The options are side by side, when there's room */
        .options {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(8rem, 1fr));
            gap: 1.5rem;
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
            this.contextWindow = typeof opt.context_window === 'number' && opt.context_window > 0 ? opt.context_window : undefined;
            this.startTools = this.session ? [...(this.session.tools ?? [])] : ['*'];
        }
        super.willUpdate(changed);
    }

    protected current(): NewSessionForm | undefined {
        return this.visible ? true : undefined;
    }

    protected renderTitle() {
        return this.session ? 'Edit conversation' : 'New conversation';
    }

    protected renderSubtitle() {
        return this.session ? 'Preferences for how the conversation works' : 'A conversation with your assistant';
    }

    // renderFooter returns Start conversation, or Save, once there are models to
    // chat with, as the body does, or nothing, while they're loading, or
    // when there are none
    protected renderFooter() {
        return this.models?.length
            ? html`<cds-button kind="primary" type="submit" ?disabled=${this.creating} @click=${this.handleSubmit}>${this.session ? 'Save' : 'Start conversation'}</cds-button>`
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
        const limit = model?.input_token_limit || undefined;
        const top = Math.log2(windowTop(limit));
        return keyed(this.opened, html`
            <form @submit=${this.handleSubmit}>
                ${this.session ? html`
                    <cds-text-input id="title" label="Title"
                        value=${this.session.title ?? ''}></cds-text-input>
                ` : nothing}
                <cds-select id="model" label-text="Assistant model" value=${chosen}
                    @cds-select-selected=${(e: CustomEvent<{ value: string }>) => { this.chosen = e.detail.value; }}>
                    ${this.models.map((model) => html`<cds-select-item value=${model.canonical}>${model.canonical}</cds-select-item>`)}
                </cds-select>
                <div>
                    <cds-slider id="context" label-text=${`Memory window: ${this.windowLabel(limit)}`}
                        min=${String(Math.log2(Math.min(minWindow, windowTop(limit))))} max=${String(top)} step="0.01"
                        .value=${live(String(windowPosition(this.contextWindow, limit)))} hide-text-input
                        .formatLabel=${(position: number) => (position >= top - 0.005 ? (limit ? tokens(limit) : 'Default') : tokens(windowOf(position)))}
                        .formatMinText=${() => tokens(Math.min(minWindow, windowTop(limit)))} .formatMaxText=${() => (limit ? tokens(limit) : 'Default')}
                        @cds-slider-changed=${(e: CustomEvent<{ value: number }>) => this.chooseWindow(Number(e.detail.value), limit)}></cds-slider>
                    <p class="note">${this.windowNote(model)}</p>
                </div>
                <cds-textarea label="Assistant instructions" helper-text="How the assistant acts, such as 'be brief', 'set humour level to 90%', or 'respond in rhyme'" rows="3"
                    value=${this.session?.system ?? ''}></cds-textarea>
                <llm-disclosure label="Advanced options">
                <cds-checkbox id="trace" ?checked=${this.session?.trace === true}>Trace: show the assistant's thinking, and the tools it calls, in the conversation</cds-checkbox>
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
                        <llm-tool-picker .tools=${this.tools} .described=${this.namespaces} .value=${this.startTools}>
                            Every tool is on, unless you turn it off. Without the <strong>llm</strong> tools, the assistant can't speak, make pictures, read scans or transcribe recordings, or read the conversation's files; without <strong>memory</strong>, it doesn't remember you from one conversation to the next; and without <strong>heartbeat</strong>, it can't set reminders, or do things on a schedule.
                        </llm-tool-picker>
                    `}
                </llm-disclosure>
            </form>
        `);
    }

    // chooseWindow sets the context window from the slider's position, or
    // the model's own, at its top, keeping the one which was set, such as
    // one from the command line, when the slider hasn't moved
    private chooseWindow(position: number, limit?: number) {
        if (Math.abs(position - windowPosition(this.contextWindow, limit)) < 0.005) {
            return;
        }
        this.contextWindow = position >= Math.log2(windowTop(limit)) - 0.005 ? undefined : windowOf(position);
    }

    // windowLabel returns the context window, as the model is asked with
    // it: what's chosen, or the model's most, when it's more, or none is
    private windowLabel(limit?: number): string {
        if (this.contextWindow && (!limit || this.contextWindow < limit)) {
            return `${tokens(this.contextWindow)} tokens`;
        }
        return limit ? `${tokens(limit)} tokens, the most this model takes` : 'Default';
    }

    // windowNote says a smaller context window is quicker, and that the
    // model's most is its own, when it isn't known
    private windowNote(model?: Model): string {
        const note = 'A smaller window is quicker, costs less, and compacts the conversation sooner.';
        return model?.input_token_limit ? note : `${note} This model doesn't say how many tokens it takes.`;
    }

    private handleSubmit(event: Event) {
        event.preventDefault();
        if (this.creating || !this.select?.value) {
            return;
        }
        const model = this.models?.find((model) => model.canonical === this.select?.value);
        const title = this.titleInput?.value.trim() ?? '';
        const system = this.system?.value.trim() ?? '';
        // Without the picker, as the model can't call tools, the tools are
        // as they were, so they're there again with a model which can
        const tools = this.picker?.value ?? this.startTools;
        const trace = this.traceBox?.checked === true;

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
        if (this.contextWindow) {
            opt.context_window = this.contextWindow;
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
            if (trace) {
                meta.trace = true;
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
        if (trace !== (session.trace === true)) {
            changes.trace = trace;
        }
        const changed: Record<string, unknown> = {};
        for (const key of ['thinking', 'thinking_effort', 'temperature', 'context_window']) {
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
