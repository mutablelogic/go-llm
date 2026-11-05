// The page which is shown for a location which isn't a page

import { customElement } from 'lit/decorators.js';
import { html } from 'lit';
import { Page } from './page';

@customElement('llm-not-found-page')
export class NotFoundPage extends Page {
    heading = 'Not found';

    render() {
        return html`${super.render()}<p>There's no page here yet.</p>`;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-not-found-page': NotFoundPage;
    }
}
