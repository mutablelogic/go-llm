// The refresh action of a page, which fires an event, so what it shows is
// loaded again

import { html } from 'lit';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Renew16 from '@carbon/icons/es/renew/16.js';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';

// eventRefresh is the name of the event fired when the refresh action is
// clicked
export const eventRefresh = 'llm-page-refresh';

// refreshAction returns the refresh button, which fires the event from the
// page
export function refreshAction(page: HTMLElement): unknown {
    return html`
        <cds-button kind="primary" size="md" @click=${() => page.dispatchEvent(new CustomEvent(eventRefresh))}>
            Refresh ${iconLoader(Renew16, { slot: 'icon' })}
        </cds-button>
    `;
}
