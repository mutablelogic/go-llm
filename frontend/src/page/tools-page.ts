// The page of the tools, and the agents, which models can call, with an
// action which loads them again

import { customElement } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import ToolKit32 from '@carbon/icons/es/tool-kit/32.js';
import { Page } from './page';
import { refreshAction } from './refresh';

@customElement('llm-tools-page')
export class ToolsPage extends Page {
    heading = 'Tools';
    description = 'The tools, and agents, which models can call';

    protected renderIcon(): unknown {
        return iconLoader(ToolKit32);
    }

    protected renderActions(): unknown {
        return refreshAction(this);
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-tools-page': ToolsPage;
    }
}
