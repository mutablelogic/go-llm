// The page of the chats, whose heading is the title of the chat which is
// open, or its model, when it has none, beneath which is its model, when
// it has a title, the tokens of its conversation, and whether someone is
// typing, or its model is replying, and whose menu starts a new chat, or
// shows the open one's tasks, or preferences, or deletes it.
// Beneath it
// is the list of chats, with the actions of the one which is open, beside
// its conversation.

import { html, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Chat32 from '@carbon/icons/es/chat/32.js';
import { Page } from './page';

// Carbon components
import '@carbon/web-components/es/components/menu-button/index.js';
import '@carbon/web-components/es/components/menu/index.js';

@customElement('llm-chat-page')
export class ChatPage extends Page {
    // The names of the events fired by the menu's items: to start a new
    // chat, and, for the one which is open, to show its tasks, its
    // preferences, or to delete it
    static readonly eventNew = 'llm-chat-page-new';
    static readonly eventTasks = 'llm-chat-page-tasks';
    static readonly eventPreferences = 'llm-chat-page-preferences';
    static readonly eventDelete = 'llm-chat-page-delete';

    heading = 'Conversations';

    // Whether a chat is open, whether it's an event's, which has no tasks,
    // or preferences, of its own, and whether it can be deleted, which it
    // can't while it's replying
    @property({ type: Boolean })
    open = false;

    @property({ type: Boolean })
    event = false;

    @property({ type: Boolean })
    deletable = true;

    protected renderIcon(): unknown {
        return iconLoader(Chat32);
    }

    protected renderActions(): unknown {
        const fire = (name: string) => () => this.dispatchEvent(new CustomEvent(name));
        return html`
            <cds-menu-button label="Manage conversation" kind="primary" size="md" menu-alignment="bottom-end">
                <cds-menu>
                    <cds-menu-item label="New conversation" @click=${fire(ChatPage.eventNew)}></cds-menu-item>
                    ${this.open && !this.event ? html`
                        <cds-menu-item label="Tasks" @click=${fire(ChatPage.eventTasks)}></cds-menu-item>
                        <cds-menu-item label="Preferences" @click=${fire(ChatPage.eventPreferences)}></cds-menu-item>
                    ` : nothing}
                    ${this.open ? html`
                        <cds-menu-item-divider></cds-menu-item-divider>
                        <cds-menu-item kind="danger" label="Delete conversation" ?disabled=${!this.deletable} @click=${fire(ChatPage.eventDelete)}></cds-menu-item>
                    ` : nothing}
                </cds-menu>
            </cds-menu-button>
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-chat-page': ChatPage;
    }
}
