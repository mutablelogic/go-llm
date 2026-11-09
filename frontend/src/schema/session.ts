// The sessions' schema, which mirrors the registry's, and the messages',
// which mirrors the providers'

// Usage is the tokens used by requests to a model, and how long they took,
// in nanoseconds
export interface Usage {
    input_tokens?: number;
    output_tokens?: number;
    cached_tokens?: number;
    reasoning_tokens?: number;
    duration?: number;
}

// SessionUsage is the tokens used by every request to a session's model,
// and the tokens of its conversation, as of the last
export interface SessionUsage extends Usage {
    input?: number;
    output?: number;
    overhead?: number;
}

// ToolCall is a model's call of a tool, by its name for the model, such as
// weatherapi__current, with its input
export interface ToolCall {
    id?: string;
    name: string;
    input?: unknown;
}

// ToolResult is the result of a tool which a model called
export interface ToolResult {
    id?: string;
    name?: string;
    content?: unknown;
    is_error?: boolean;
    usage?: Usage;
}

// ContentBlock is a part of a message, which is one of its text, its
// thinking, an attachment, a tool call or a tool's result
export interface ContentBlock {
    text?: string;
    thinking?: string;
    // What the system says of the message, for the model, rather than the
    // person, such as where it was sent
    annotation?: string;
    attachment?: { type?: string; url?: string };
    tool_call?: ToolCall;
    tool_result?: ToolResult;
}

// Message is a message of a conversation, from the user, the assistant, or
// a tool, with its tokens, and why the model stopped, when it's a reply
export interface Message {
    role?: string;
    content: ContentBlock[];
    tokens?: number;
    result?: string;
    time?: string;
    event?: MessageEvent;
    marker?: Marker;
}

// Marker marks where a conversation's context starts, as it was compacted,
// with a summary of what came before it, which is written in the background
export interface Marker {
    id: string;
    reason: 'asked' | 'full';
    status: 'pending' | 'summarized' | 'failed';
    summary?: string;
    title?: string;
    error?: string;
}

// MessageEvent is the event, such as a heartbeat which fired, whose outcome
// a message is, which was delivered to the chat it's for
export interface MessageEvent {
    id: string;
    title?: string;
}

// SessionStatus is whether someone is typing in a session, or its model is
// replying, or neither
export type SessionStatus = 'idle' | 'typing' | 'replying';

// Session is a conversation with a model, without its messages, which are
// listed on their own
export interface Session {
    id: string;
    serial: number;
    created?: string;
    modified?: string;
    // What it's about, for people, such as an event's title, which its
    // session takes
    title?: string;
    model?: string;
    system?: string;
    tools?: string[];
    opt?: Record<string, unknown>;
    schema?: unknown;
    usage?: SessionUsage;
    status?: SessionStatus;
    // Whether it's an event's session, such as a heartbeat's, which only the
    // server sets, which isn't chatted in
    event?: boolean;
    forgotten?: number;
}

// SessionMeta is the settings of a session, which it's created with
export interface SessionMeta {
    title?: string;
    model?: string;
    system?: string;
    tools?: string[];
    opt?: Record<string, unknown>;
}

// SessionList is a page of the sessions, and how many there are
export interface SessionList {
    count: number;
    body?: Session[];
}

// MessageList is the messages of a session, oldest first, and how many
// there are
export interface MessageList {
    count: number;
    offset?: number;
    body?: Message[];
}

// SessionEvent is the data of an event of a session: its new serial, or its
// status
export interface SessionEvent {
    serial?: number;
    status?: SessionStatus;
}

// ChatResponse is what was added to a session, with its serial, and the
// usage of the reply
export interface ChatResponse {
    id: string;
    serial: number;
    messages?: Message[];
    usage?: SessionUsage;
}

// Progress is how a tool is getting on, which replaces the one before
export interface Progress {
    status?: string;
    percent?: number;
    progress?: number;
    total?: number;
    message?: string;
}

// isUser returns whether the message is the person's, which it is when it
// has no role, as the API takes it to be
export function isUser(message: Message): boolean {
    return (message.role ?? 'user') === 'user';
}

// messageText returns the text of a message, without its thinking, tool
// calls, or tools' results
export function messageText(message: Message): string {
    return message.content.map((block) => block.text ?? '').join('');
}

// tokens returns the tokens of a session's conversation
export function sessionTokens(session: Session): number {
    const usage = session.usage ?? {};
    return (usage.input ?? 0) + (usage.output ?? 0) + (usage.overhead ?? 0);
}

// canonicalTool returns the canonical name of a tool, from its name for a
// model, such as weatherapi:current for weatherapi__current
export function canonicalTool(name: string): string {
    const separator = name.indexOf('__');
    return separator < 0 ? name : name.slice(0, separator) + ':' + name.slice(separator + 2);
}

// reminderText is a text block which is all one system reminder, as a
// message's annotation was stored before annotations, such as
// <system-reminder>Sent on Telegram</system-reminder>
const reminderText = /^\s*<system-reminder>\s*([\s\S]*?)\s*<\/system-reminder>\s*$/;

// annotationOf returns what the system says of a message, in the content
// block, which is its annotation, or a text block which is all a system
// reminder, as one was stored before annotations, or undefined
export function annotationOf(block: ContentBlock): string | undefined {
    if (block.annotation?.trim()) {
        return block.annotation.trim();
    }
    const match = block.text?.match(reminderText);
    return match ? match[1] : undefined;
}
