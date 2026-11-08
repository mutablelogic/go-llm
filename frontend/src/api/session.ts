// The sessions' API

import { request, requestStream } from './client';
import type { ChatResponse, Message, MessageList, Progress, Session, SessionList, SessionMeta, SessionStatus } from '../schema/session';

// listSessions returns a page of the sessions, most recently modified first
export function listSessions(offset: number, limit: number, signal?: AbortSignal): Promise<SessionList> {
    return request<SessionList>('GET', '/llm/session', { query: { offset, limit }, signal });
}

// getSession returns the session, without its messages
export function getSession(id: string, signal?: AbortSignal): Promise<Session> {
    return request<Session>('GET', '/llm/session/' + id, { signal });
}

// listMessages returns the session's messages, oldest first
export function listMessages(id: string, signal?: AbortSignal): Promise<MessageList> {
    return request<MessageList>('GET', `/llm/session/${id}/messages`, { signal });
}

// compactSession compacts the session: its context starts at a marker,
// whose summary of what came before is written in the background
export function compactSession(id: string): Promise<Session> {
    return request<Session>('POST', `/llm/session/${id}/compact`);
}

// createSession creates a session with the settings
export function createSession(meta: SessionMeta): Promise<Session> {
    return request<Session>('POST', '/llm/session', { body: meta });
}

// updateSession changes the session's settings: those which are set are
// replaced, an empty model, or system prompt, and no tools, clear them, and
// an option which is null is removed
export function updateSession(id: string, meta: SessionMeta): Promise<Session> {
    return request<Session>('PATCH', '/llm/session/' + id, { body: meta });
}

// deleteSession deletes the session
export function deleteSession(id: string): Promise<Session> {
    return request<Session>('DELETE', '/llm/session/' + id);
}

// setStatus says someone is typing in the session, or has stopped
export function setStatus(id: string, status: Exclude<SessionStatus, 'replying'>): Promise<Session> {
    return request<Session>('PUT', `/llm/session/${id}/status`, { body: { status } });
}

// ChatStream is what's called as a reply is streamed: with the pieces of its
// text, and its thinking, the lines tools log, and how tools are getting on
export interface ChatStream {
    onText?: (text: string) => void;
    onThinking?: (text: string) => void;
    onLog?: (line: string) => void;
    onProgress?: (progress: Progress) => void;
}

// chat adds the message to the session, and returns what was added, which
// is its model's reply, and the results of the tools it called, as it's
// streamed. It throws the error of an error event.
export async function chat(id: string, message: Message, stream: ChatStream, signal?: AbortSignal): Promise<ChatResponse> {
    let response: ChatResponse | undefined;
    let error: string | undefined;
    await requestStream('POST', '/llm/session/' + id, (event, data) => {
        const piece = data as { text?: string; error?: string };
        switch (event) {
            case 'text':
                stream.onText?.(piece.text ?? '');
                break;
            case 'thinking':
                stream.onThinking?.(piece.text ?? '');
                break;
            case 'log':
                stream.onLog?.(piece.text ?? '');
                break;
            case 'progress':
                stream.onProgress?.(data as Progress);
                break;
            case 'result':
                response = data as ChatResponse;
                break;
            case 'error':
                error = piece.error;
                break;
        }
    }, { body: { messages: [message] }, signal });
    if (error !== undefined) {
        throw new Error(error);
    } else if (!response) {
        throw new Error('the reply ended before it was complete');
    }
    return response;
}
