// Markdown, such as a model's reply, rendered as HTML, which is sanitized,
// as a model's reply can have anything in it, with links which open in a new
// tab, and the styles of what it renders

import { css } from 'lit';
import { unsafeHTML } from 'lit/directives/unsafe-html.js';
import { marked } from 'marked';
import DOMPurify from 'dompurify';

// Links open in a new tab, without the page they're opened from
DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    if (node instanceof HTMLAnchorElement) {
        node.setAttribute('target', '_blank');
        node.setAttribute('rel', 'noopener noreferrer');
    }
});

// markdown returns the Markdown as HTML, which is sanitized
export function markdown(text: string): unknown {
    const html = marked.parse(text, { async: false, gfm: true });
    return unsafeHTML(DOMPurify.sanitize(html));
}

// markdownStyles are the styles of the HTML which markdown returns, within
// an element with the class markdown
export const markdownStyles = css`
    .markdown {
        line-height: 1.5;
        overflow-wrap: anywhere;
    }

    .markdown > :first-child {
        margin-top: 0;
    }

    .markdown > :last-child {
        margin-bottom: 0;
    }

    .markdown p,
    .markdown ul,
    .markdown ol,
    .markdown pre,
    .markdown table,
    .markdown blockquote {
        margin: 0 0 0.75rem;
    }

    .markdown ul,
    .markdown ol {
        padding-left: 1.5rem;
    }

    .markdown h1,
    .markdown h2,
    .markdown h3,
    .markdown h4 {
        margin: 1rem 0 0.5rem;
        font-size: 1rem;
        font-weight: 600;
    }

    .markdown code {
        font-family: var(--cds-code-01-font-family, 'IBM Plex Mono', monospace);
        font-size: 0.8125rem;
        background: var(--cds-layer-02);
        padding: 0 0.25rem;
    }

    .markdown pre {
        padding: 0.75rem;
        overflow-x: auto;
        background: var(--cds-layer-02);
    }

    .markdown pre code {
        padding: 0;
        background: none;
    }

    .markdown blockquote {
        padding-left: 0.75rem;
        border-left: 3px solid var(--cds-border-subtle-01);
        color: var(--cds-text-secondary);
    }

    .markdown table {
        border-collapse: collapse;
    }

    .markdown th,
    .markdown td {
        padding: 0.25rem 0.5rem;
        border: 1px solid var(--cds-border-subtle-01);
        text-align: left;
    }

    .markdown a {
        color: var(--cds-link-primary);
    }
`;
