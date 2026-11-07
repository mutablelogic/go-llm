// Markdown, such as a model's reply, rendered as HTML, which is sanitized,
// as a model's reply can have anything in it, with links which open in a new
// tab, and a QR code under a link which connects a Telegram chat, so it can
// be scanned with a phone, and the styles of what it renders

import { css } from 'lit';
import { unsafeHTML } from 'lit/directives/unsafe-html.js';
import { marked } from 'marked';
import DOMPurify from 'dompurify';
import { renderSVG } from 'uqr';

// connectLink is a link which connects a Telegram chat to a session, as the
// messaging:connect tool returns it, such as https://t.me/bot?start=code
const connectLink = /^https:\/\/t\.me\/[A-Za-z0-9_]{5,32}\?start=[A-Za-z0-9_-]{1,64}$/;

// A link which connects a chat has its QR code under it, which is made in
// the browser, as an image, so it's sanitized as one. Other links are
// rendered as they are.
marked.use({
    renderer: {
        link(token) {
            if (!connectLink.test(token.href)) {
                return false;
            }
            const href = escapeAttribute(token.href);
            const svg = renderSVG(token.href, { border: 2 }).replace('<svg ', '<svg shape-rendering="crispEdges" ');
            const src = 'data:image/svg+xml;base64,' + btoa(svg);
            return `<a href="${href}">${this.parser.parseInline(token.tokens)}</a>`
                + `<img class="qr" src="${src}" alt="A QR code of the link, to scan with a phone" width="160" height="160">`;
        },
    },
});

// escapeAttribute returns the text, escaped as an attribute's value
function escapeAttribute(text: string): string {
    return text.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// Links open in a new tab, without the page they're opened from
DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    if (node instanceof HTMLAnchorElement) {
        node.setAttribute('target', '_blank');
        node.setAttribute('rel', 'noopener noreferrer');
    }
});

// markdown returns the Markdown as HTML, which is sanitized. The resolve
// function, when there is one, returns the URL a link, or an image, is to,
// such as the URL of a resource's content, for its resource:// URI, or
// undefined, to keep it, so only links, and images, are resolved, rather
// than the same text in code, or prose.
export function markdown(text: string, resolve?: (href: string) => string | undefined): unknown {
    const html = marked.parse(text, {
        async: false,
        gfm: true,
        walkTokens: resolve ? (token) => {
            if ((token.type === 'link' || token.type === 'image') && token.href) {
                token.href = resolve(token.href) ?? token.href;
            }
        } : undefined,
    });
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

    .markdown img.qr {
        display: block;
        width: 10rem;
        height: 10rem;
        margin: 0.5rem 0;
        background: #fff;
        image-rendering: pixelated;
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
