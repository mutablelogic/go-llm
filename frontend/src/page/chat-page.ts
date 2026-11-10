// The page of the chats, whose heading is the title of the chat which is
// open, or its model, when it has none, beneath which is its model, when
// it has a title, the tokens of its conversation, and whether someone is
// typing, or its model is replying, and whose action starts a new chat.
// Beneath it
// is the list of chats, with the actions of the one which is open, beside
// its conversation.

import { html } from 'lit';
import { customElement } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Chat32 from '@carbon/icons/es/chat/32.js';
import Add16 from '@carbon/icons/es/add/16.js';
import { Page } from './page';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';

@customElement('llm-chat-page')
export class ChatPage extends Page {
    // The name of the event fired to start a new chat
    static readonly eventNew = 'llm-chat-page-new';

    heading = 'Conversations';

    protected renderIcon(): unknown {
        return iconLoader(Chat32);
    }

    protected renderActions(): unknown {
        return html`
            <cds-button kind="primary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(ChatPage.eventNew))}>
                Add conversation ${iconLoader(Add16, { slot: 'icon' })}
            </cds-button>
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-chat-page': ChatPage;
    }
}
