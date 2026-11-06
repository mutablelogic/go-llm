// A player of audio, and video, in the side navigation, above its footer,
// which plays what's chosen to be played, such as speech which a tool
// returned, or a link to it in a reply, so it's played in the page, rather
// than in a new tab, and keeps playing as the chat changes. It's hidden until
// something is played, and when it's closed.

import { LitElement, css, html, nothing, type PropertyValues } from 'lit';
import { customElement, property, query } from 'lit/decorators.js';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import Close16 from '@carbon/icons/es/close/16.js';
import Music16 from '@carbon/icons/es/music/16.js';
import Video16 from '@carbon/icons/es/video/16.js';

// Carbon components
import '@carbon/web-components/es/components/icon-button/index.js';

// Media is what's played: its URL, its name, and its type, such as
// audio/wav, or video/mp4
export interface Media {
    url: string;
    name: string;
    type?: string;
}

@customElement('llm-media-player')
export class MediaPlayer extends LitElement {
    // The name of the event which plays media, whose detail is the media,
    // which bubbles out of the shadow roots it's fired in
    static readonly eventPlay = 'llm-media-play';

    // What's played, or none, when it's hidden
    @property({ attribute: false })
    media?: Media;

    // Whether something is played, which is reflected, so the side
    // navigation's items scroll above it
    @property({ type: Boolean, reflect: true })
    playing = false;

    @query('audio, video')
    private element?: HTMLMediaElement;

    static styles = css`
        :host {
            position: absolute;
            right: 0;
            bottom: calc(3rem + 1px);
            left: 0;
            display: none;
            padding: 0.5rem 0.5rem 0.75rem 1rem;
            border-top: 1px solid var(--cds-border-subtle);
            color: var(--cds-text-secondary);
            font-size: 0.75rem;
            letter-spacing: 0.32px;
        }

        :host([playing]) {
            display: block;
        }

        .title {
            display: flex;
            align-items: center;
            gap: 0.375rem;
            margin-bottom: 0.25rem;
        }

        .title svg {
            flex: none;
            fill: currentColor;
        }

        .name {
            flex: 1;
            min-width: 0;
            overflow: hidden;
            color: var(--cds-text-primary);
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        audio,
        video {
            display: block;
            width: calc(100% - 0.5rem);
            color-scheme: dark;
        }

        video {
            max-height: 10rem;
            background: black;
        }
    `;

    // play plays the media, from its start, replacing what's played
    play(media: Media) {
        this.media = media;
    }

    willUpdate(changed: PropertyValues<this>) {
        if (changed.has('media')) {
            this.playing = this.media !== undefined;
        }
    }

    updated(changed: PropertyValues<this>) {
        // The media which is chosen is played, once its element is rendered
        if (changed.has('media') && this.media) {
            this.element?.play().catch(() => {
                // The browser didn't let it play, so its controls play it
            });
        }
    }

    render() {
        const media = this.media;
        if (!media) {
            return nothing;
        }
        const video = (media.type ?? '').toLowerCase().startsWith('video/');
        return html`
            <div class="title">
                ${iconLoader(video ? Video16 : Music16)}
                <span class="name" title=${media.name}>${media.name}</span>
                <cds-icon-button kind="ghost" size="sm" align="top-right" @click=${this.close}>
                    ${iconLoader(Close16, { slot: 'icon' })}
                    <span slot="tooltip-content">Close</span>
                </cds-icon-button>
            </div>
            ${video
                ? html`<video controls preload="metadata" src=${media.url}></video>`
                : html`<audio controls preload="metadata" src=${media.url}></audio>`}
        `;
    }

    // close stops what's played, and hides the player
    private close() {
        this.element?.pause();
        this.media = undefined;
    }
}

// playable returns whether media of the type is played by the player, which
// is audio, and video
export function playable(type?: string): boolean {
    const mediaType = (type ?? '').split(';')[0].trim().toLowerCase();
    return mediaType.startsWith('audio/') || mediaType.startsWith('video/');
}

declare global {
    interface HTMLElementTagNameMap {
        'llm-media-player': MediaPlayer;
    }
}
