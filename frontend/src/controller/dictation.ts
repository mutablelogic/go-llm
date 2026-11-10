// The dictation controller, which dictates a message into the chat's box,
// with the browser's speech recognition, such as Chrome's, Edge's and
// Safari's, which recognises speech as it's spoken, in the page's language.
// The words which are still being recognised are shown, and replaced, until
// they're final. It's started, and stopped, by the box's microphone button,
// which isn't shown when the browser can't recognise speech, and stops when
// the message is sent. Chrome sends the speech to Google to recognise it.

import { ChatView } from '../component/chat-view';
import { DictateButton } from '../component/dictate-button';
import type { NotificationController } from './notifications';

// The browser's speech recognition, which the DOM's types don't have
interface Recognition extends EventTarget {
    lang: string;
    continuous: boolean;
    interimResults: boolean;
    start(): void;
    stop(): void;
    abort(): void;
    onresult: ((event: RecognitionEvent) => void) | null;
    onerror: ((event: { error: string; message?: string }) => void) | null;
    onend: (() => void) | null;
}

interface RecognitionEvent {
    resultIndex: number;
    results: ArrayLike<{ isFinal: boolean; 0: { transcript: string } }>;
}

type RecognitionConstructor = new () => Recognition;

// recognitionConstructor returns the browser's speech recognition, or
// undefined, when it hasn't one, such as Firefox
function recognitionConstructor(): RecognitionConstructor | undefined {
    const w = window as unknown as { SpeechRecognition?: RecognitionConstructor; webkitSpeechRecognition?: RecognitionConstructor };
    return w.SpeechRecognition ?? w.webkitSpeechRecognition;
}

// errors are what's said when speech can't be recognised, by the browser's
// error, apart from those which aren't worth saying, such as when nothing was
// said, or it was stopped
const errors: Record<string, string> = {
    'not-allowed': "The microphone isn't allowed: allow it in the browser's settings for this page, to dictate",
    'service-not-allowed': "The browser doesn't allow speech to be recognised here",
    'audio-capture': "There's no microphone to dictate with",
    'network': "Speech couldn't be recognised, as the browser's service couldn't be reached",
    'language-not-supported': "The browser can't recognise speech in this page's language",
};

export class DictationController {
    private readonly view: ChatView | null;
    private readonly notifications: NotificationController;
    private readonly Recognition?: RecognitionConstructor;
    private recognition?: Recognition;

    constructor(view: ChatView | null, notifications: NotificationController) {
        this.view = view;
        this.notifications = notifications;
        this.Recognition = recognitionConstructor();
        if (!view || !this.Recognition) {
            return;
        }
        view.dictation = true;
        view.addEventListener(DictateButton.eventToggle, () => this.toggle());
        view.addEventListener(ChatView.eventSend, () => this.stop());
    }

    // supported returns whether the browser recognises speech
    get supported(): boolean {
        return this.Recognition !== undefined;
    }

    // toggle starts dictating, or stops, when it is
    toggle() {
        if (this.recognition) {
            this.stop();
        } else {
            this.start();
        }
    }

    // start dictates into the box, as the person speaks, until it's stopped,
    // or the browser stops listening, such as after a silence
    start() {
        if (!this.Recognition || !this.view || this.recognition) {
            return;
        }
        const recognition = new this.Recognition();
        recognition.lang = document.documentElement.lang || navigator.language;
        recognition.continuous = true;
        recognition.interimResults = true;
        recognition.onresult = (event) => {
            for (let i = event.resultIndex; i < event.results.length; i++) {
                const result = event.results[i];
                this.view?.dictate(result[0].transcript, result.isFinal);
            }
        };
        recognition.onerror = (event) => {
            const message = errors[event.error];
            if (message) {
                this.notifications.error(message, event.message ?? '');
            }
        };
        recognition.onend = () => {
            if (this.recognition === recognition) {
                this.recognition = undefined;
                this.view?.dictated();
            }
        };
        this.recognition = recognition;
        this.view.listening = true;
        try {
            recognition.start();
        } catch (error) {
            this.recognition = undefined;
            this.view.dictated();
            this.notifications.error("Dictation couldn't be started", error);
        }
    }

    // stop stops dictating, keeping what was recognised
    stop() {
        const recognition = this.recognition;
        if (!recognition) {
            return;
        }
        this.recognition = undefined;
        recognition.stop();
        this.view?.dictated();
    }
}
