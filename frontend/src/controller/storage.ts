// The storage controller, which keeps the defaults of the UI in the
// browser, between visits, such as the chat which was last open. Each value
// is kept as JSON, by its key, with a prefix, so it's apart from those of
// other apps on the same host. When the browser doesn't keep them, such as
// in a private window which is full, the defaults are lost, and nothing
// fails.

// StorageKeys are the defaults, by their keys, and their types
export interface StorageKeys {
    // The identifier of the chat which was last open
    chat: string;
}

export class StorageController {
    private readonly prefix: string;

    constructor(prefix = 'llm.') {
        this.prefix = prefix;
    }

    // get returns the value of the key, or undefined, when there's none, or
    // it can't be read
    get<K extends keyof StorageKeys>(key: K): StorageKeys[K] | undefined {
        try {
            const value = localStorage.getItem(this.prefix + key);
            return value === null ? undefined : (JSON.parse(value) as StorageKeys[K]);
        } catch {
            return undefined;
        }
    }

    // set sets the value of the key, or removes it, when it's undefined
    set<K extends keyof StorageKeys>(key: K, value?: StorageKeys[K]) {
        try {
            if (value === undefined) {
                localStorage.removeItem(this.prefix + key);
            } else {
                localStorage.setItem(this.prefix + key, JSON.stringify(value));
            }
        } catch {
            // The browser doesn't keep it
        }
    }

    // delete removes the key, when its value is the value, or whatever it
    // is, without one
    delete<K extends keyof StorageKeys>(key: K, value?: StorageKeys[K]) {
        if (value === undefined || this.get(key) === value) {
            this.set(key);
        }
    }
}
