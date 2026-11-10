// A button which starts dictating a message, with the browser's speech
// recognition, and stops it, which is secondary, as the attach button is, and
// primary, and filled, while it's listening. It fires
// an event, which the dictation controller starts, or stops, dictation for.

import { LitElement, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Microphone16 from '@carbon/icons/es/microphone/16.js';
import MicrophoneFilled16 from '@carbon/icons/es/microphone--filled/16.js';

// Carbon components
import '@carbon/web-components/es/components/icon-button/index.js';

@customElement('llm-dictate-button')
export class DictateButton extends LitElement {
    // The name of the event fired to start dictating, or stop, which bubbles
    // out of the chat's box, to the dictation controller
    static readonly eventToggle = 'llm-dictate-toggle';

    // The name of the event fired to stop dictating, such as when the box is
    // typed in, which bubbles out of the chat's box, to the controller
    static readonly eventStop = 'llm-dictate-stop';

    // Whether it's listening, as the person speaks
    @property({ type: Boolean, reflect: true })
    listening = false;

    @property({ type: Boolean, reflect: true })
    disabled = false;

    render() {
        const label = this.listening ? 'Stop dictating' : 'Dictate';
        return html`
            <cds-icon-button kind=${this.listening ? 'primary' : 'secondary'} size="md" align="top" ?disabled=${this.disabled}
                aria-pressed=${String(this.listening)}
                @click=${() => this.dispatchEvent(new CustomEvent(DictateButton.eventToggle, { bubbles: true, composed: true }))}>
                ${iconLoader(this.listening ? MicrophoneFilled16 : Microphone16, { slot: 'icon' })}
                <span slot="tooltip-content">${label}</span>
            </cds-icon-button>
        `;
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-dictate-button': DictateButton;
    }
}
