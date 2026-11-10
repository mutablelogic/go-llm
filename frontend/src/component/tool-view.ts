// A panel which shows a tool: its description, and the JSON schemas of its
// input and output

import { css, html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { Panel } from './panel';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import ToolKit16 from '@carbon/icons/es/tool-kit/16.js';
import { namespaceIcon } from './namespace-icon';
import { toolNamespace } from '../schema/tool';
import type { Tool } from '../schema/tool';

@customElement('llm-tool-view')
export class ToolView extends Panel<Tool> {
    // The tool which is shown, or none, when the panel is closed
    @property({ attribute: false })
    tool?: Tool;

    static styles = [Panel.styles, css`
        .description {
            white-space: pre-wrap;
        }

        pre {
            margin: 0 0 1.5rem;
            padding: 0.75rem;
            overflow-x: auto;
            background: var(--cds-layer-02);
        }
    `];

    protected current() {
        return this.tool;
    }

    protected renderIcon(tool: Tool) {
        return namespaceIcon(toolNamespace(tool)) ?? iconLoader(ToolKit16);
    }

    protected renderTitle(tool: Tool) {
        return tool.name;
    }

    protected renderSubtitle(tool: Tool) {
        return tool.canonical;
    }

    protected renderBody(tool: Tool) {
        return html`
            ${tool.description ? html`<p class="description">${tool.description.trim()}</p>` : nothing}
            ${tool.input ? html`<h3>Input</h3><pre><code>${JSON.stringify(tool.input, null, 2)}</code></pre>` : nothing}
            ${tool.output ? html`<h3>Output</h3><pre><code>${JSON.stringify(tool.output, null, 2)}</code></pre>` : nothing}
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-tool-view': ToolView;
    }
}
