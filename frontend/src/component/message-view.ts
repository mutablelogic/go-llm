// A panel on the right of the page, which shows a message's details: who
// it's from, its tokens, why the model stopped, and, for a tool's result,
// how long the tool took, and the tokens it used, such as an agent's

import { html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { canonicalTool, type Message, type Usage } from '../schema/session';
import { formatDuration } from '../format';
import { Panel } from './panel';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Information16 from '@carbon/icons/es/information/16.js';

@customElement('llm-message-view')
export class MessageView extends Panel<Message> {
    // The message, which is undefined when the panel is closed, and who it's
    // from, by name
    @property({ attribute: false })
    message?: Message;

    @property({ attribute: false })
    from = '';

    protected current() {
        return this.message;
    }

    protected renderIcon() {
        return iconLoader(Information16);
    }

    protected renderTitle() {
        return this.from;
    }

    protected renderSubtitle(message: Message) {
        return message.role || 'user';
    }

    protected renderBody(message: Message) {
        const results = message.content.flatMap((block) => block.tool_result ? [block.tool_result] : []);
        const calls = message.content.flatMap((block) => block.tool_call ? [block.tool_call] : []);
        return html`
            <h3>Message</h3>
            <dl>
                <dt>Role</dt><dd>${message.role ?? 'user'}</dd>
                ${message.tokens ? html`<dt>Tokens</dt><dd>${message.tokens.toLocaleString()}</dd>` : nothing}
                ${message.result ? html`<dt>Stopped</dt><dd>${stopped(message.result)}</dd>` : nothing}
                ${calls.length ? html`<dt>Called</dt><dd>${calls.map((call) => canonicalTool(call.name)).join(', ')}</dd>` : nothing}
            </dl>
            ${results.map((result) => html`
                <h3>${canonicalTool(result.name ?? 'Tool')}${result.is_error ? ' failed' : ''}</h3>
                ${result.usage ? renderUsage(result.usage) : html`<p>No usage</p>`}
            `)}
        `;
    }
}

// renderUsage returns how long a tool took, and the tokens it used
function renderUsage(usage: Usage) {
    const rows: [string, string | undefined][] = [
        ['Took', formatDuration(usage.duration) || (usage.duration ? `${Math.round(usage.duration / 1e6)}ms` : undefined)],
        ['Input tokens', usage.input_tokens?.toLocaleString()],
        ['Output tokens', usage.output_tokens?.toLocaleString()],
        ['Cached tokens', usage.cached_tokens?.toLocaleString()],
        ['Reasoning tokens', usage.reasoning_tokens?.toLocaleString()],
    ];
    return html`<dl>${rows.filter(([, value]) => value).map(([label, value]) => html`<dt>${label}</dt><dd>${value}</dd>`)}</dl>`;
}

// stopped returns why the model stopped generating the message
function stopped(result: string): string {
    const results: Record<string, string> = {
        stop: 'It finished',
        max_tokens: 'It reached the most tokens',
        blocked: 'It was blocked',
        tool_call: 'It called a tool',
        error: 'An error',
        max_iterations: 'It called tools too many times',
    };
    return results[result] ?? result;
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-message-view': MessageView;
    }
}
