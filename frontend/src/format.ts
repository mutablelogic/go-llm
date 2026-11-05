// Formatting of values for display

// The units of durations, in nanoseconds, from largest to smallest
const durationUnits: [string, number][] = [
    ['d', 24 * 60 * 60 * 1e9],
    ['h', 60 * 60 * 1e9],
    ['m', 60 * 1e9],
    ['s', 1e9],
];

// formatDuration returns a duration in nanoseconds, as Go's time.Duration is,
// in its largest units, such as 1h30m, or an empty string when there's none
export function formatDuration(ns?: number): string {
    if (!ns || ns <= 0) {
        return '';
    }
    let result = '';
    let rest = ns;
    for (const [unit, size] of durationUnits) {
        const n = Math.floor(rest / size);
        if (n > 0) {
            result += `${n}${unit}`;
            rest -= n * size;
        }
    }
    return result || `${Math.round(ns / 1e6)}ms`;
}

// formatTime returns an RFC 3339 time, as the time it is locally, or an
// empty string when there's none
export function formatTime(value?: string): string {
    return value ? new Date(value).toLocaleString() : '';
}



// formatAgo returns an RFC 3339 time, relative to now, as minutes ago, such
// as 5 minutes ago, when it's less than an hour ago, or hours ago, such as
// 3 hours ago, when it's less than a day ago, or else as the time it is
// locally, as formatTime does, which is also so for a time to come, or an
// empty string when there's none
export function formatAgo(value?: string, now: Date = new Date()): string {
    if (!value) {
        return '';
    }
    const ago = now.getTime() - new Date(value).getTime();
    const minutes = Math.floor(ago / 60000);
    if (ago < 0 || minutes >= 24 * 60) {
        return formatTime(value);
    } else if (minutes < 1) {
        return 'just now';
    } else if (minutes < 60) {
        return minutes === 1 ? '1 minute ago' : `${minutes} minutes ago`;
    }
    const hours = Math.floor(minutes / 60);
    return hours === 1 ? '1 hour ago' : `${hours} hours ago`;
}

