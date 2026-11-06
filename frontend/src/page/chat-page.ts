// The page of the chats, whose heading is the model of the chat which is
// open, with the tokens of its conversation, and whether someone is typing,
// or its model is replying, and whose actions start a new chat, and show the
// events of, edit, and delete, the one which is open. Beneath it is the list of chats, beside the
// conversation of the one which is open.

import { html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Chat32 from '@carbon/icons/es/chat/32.js';
import Add16 from '@carbon/icons/es/add/16.js';
import TrashCan16 from '@carbon/icons/es/trash-can/16.js';
import Edit16 from '@carbon/icons/es/edit/16.js';
import Notification16 from '@carbon/icons/es/notification/16.js';
import { Page } from './page';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';

@customElement('llm-chat-page')
export class ChatPage extends Page {
    // The names of the events fired to start a new chat, and to delete the
    // chat which is open
    static readonly eventNew = 'llm-chat-page-new';
    static readonly eventDelete = 'llm-chat-page-delete';
    static readonly eventEdit = 'llm-chat-page-edit';
    static readonly eventEvents = 'llm-chat-page-events';

    heading = 'Chat';

    // Whether there's a chat open, which can be deleted
    @property({ type: Boolean })
    deletable = false;

    protected renderIcon(): unknown {
        return iconLoader(Chat32);
    }

    protected renderActions(): unknown {
        return html`
            ${this.deletable ? html`
                <cds-button kind="secondary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(ChatPage.eventEvents))}>
                    Events ${iconLoader(Notification16, { slot: 'icon' })}
                </cds-button>
                <cds-button kind="secondary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(ChatPage.eventEdit))}>
                    Edit ${iconLoader(Edit16, { slot: 'icon' })}
                </cds-button>
                <cds-button kind="secondary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(ChatPage.eventDelete))}>
                    Delete ${iconLoader(TrashCan16, { slot: 'icon' })}
                </cds-button>
            ` : nothing}
            <cds-button kind="primary" size="md" @click=${() => this.dispatchEvent(new CustomEvent(ChatPage.eventNew))}>
                New chat ${iconLoader(Add16, { slot: 'icon' })}
            </cds-button>
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-chat-page': ChatPage;
    }
}
