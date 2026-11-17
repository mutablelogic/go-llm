// A Markdown document, rendered as HTML, which is sanitized, with the
// styles of Markdown, such as of a reply, for a page, or a panel, whose
// content isn't in a shadow root which has them

import { LitElement, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { markdown, markdownStyles } from './markdown';

@customElement('llm-markdown-view')
export class MarkdownView extends LitElement {
    static override styles = [markdownStyles];

    // The Markdown, which is rendered
    @property()
    text = '';

    protected override render() {
        return html`<div class="markdown">${markdown(this.text)}</div>`;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-markdown-view': MarkdownView;
    }
}
