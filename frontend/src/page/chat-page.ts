// The page of the chats, whose heading is the title of the chat which is
// open, or its model, when it has none, beneath which is its model, when
// it has a title, the tokens of its conversation, and whether someone is
// typing, or its model is replying, and whose actions are Add, which starts
// a new chat, and Edit, a menu of the open one's tasks, and preferences,
// summarising it, and deleting it.
// Beneath it
// is the list of chats, with the actions of the one which is open, beside
// its conversation.

import { css, html, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Chat32 from '@carbon/icons/es/chat/32.js';
import Add16 from '@carbon/icons/es/add/16.js';
import { Page } from './page';

// Carbon components
import '@carbon/web-components/es/components/button/index.js';
import '@carbon/web-components/es/components/menu-button/index.js';
import '@carbon/web-components/es/components/menu/index.js';

@customElement('llm-chat-page')
export class ChatPage extends Page {
    // The names of the events fired by its actions: to start a new chat,
    // and, of the Edit menu, for the one which is open, to show its tasks,
    // its preferences, to summarise it, or to delete it
    static readonly eventNew = 'llm-chat-page-new';
    static readonly eventTasks = 'llm-chat-page-tasks';
    static readonly eventPreferences = 'llm-chat-page-preferences';
    static readonly eventCompact = 'llm-chat-page-compact';
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

    // Whether Edit is marked, as an event for the chat which is open was just
    // created, which it is for a few seconds, and the timer which ends it
    @state()
    private alerted = false;

    static readonly alertDuration = 5000;
    private alertTimer?: ReturnType<typeof setTimeout>;

    static styles = [Page.styles, css`
        /* Edit, marked with a dot, as an event was just created */
        .edit {
            position: relative;
        }

        .edit .dot {
            position: absolute;
            top: 0.375rem;
            right: 0.375rem;
            width: 0.5rem;
            height: 0.5rem;
            border-radius: 50%;
            background: var(--cds-support-error);
            pointer-events: none;
        }
    `];

    // alert marks Edit, for a few seconds, as an event for the chat which is
    // open was just created, whose reply its Tasks show
    alert() {
        this.alerted = true;
        clearTimeout(this.alertTimer);
        this.alertTimer = setTimeout(() => {
            this.alerted = false;
        }, ChatPage.alertDuration);
    }

    protected renderIcon(): unknown {
        return iconLoader(Chat32);
    }

    protected renderActions(): unknown {
        const fire = (name: string) => () => this.dispatchEvent(new CustomEvent(name));
        return html`
            ${this.open ? html`
                <span class="edit">
                    <cds-menu-button label="Edit" kind="tertiary" size="md" menu-alignment="bottom-end">
                        <cds-menu>
                            ${this.event ? nothing : html`
                                <cds-menu-item label="Tasks" @click=${fire(ChatPage.eventTasks)}></cds-menu-item>
                                <cds-menu-item label="Conversation preferences" @click=${fire(ChatPage.eventPreferences)}></cds-menu-item>
                            `}
                            <cds-menu-item label="Summarise conversation" @click=${fire(ChatPage.eventCompact)}></cds-menu-item>
                            <cds-menu-item-divider></cds-menu-item-divider>
                            <cds-menu-item kind="danger" label="Delete conversation" ?disabled=${!this.deletable} @click=${fire(ChatPage.eventDelete)}></cds-menu-item>
                        </cds-menu>
                    </cds-menu-button>
                    ${this.alerted ? html`<span class="dot" title="A task just ran"></span>` : nothing}
                </span>
            ` : nothing}
            <cds-button kind="primary" size="md" @click=${fire(ChatPage.eventNew)}>
                Add ${iconLoader(Add16, { slot: 'icon' })}
            </cds-button>
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-chat-page': ChatPage;
    }
}
