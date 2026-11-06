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

// The units of sizes, in bytes, from smallest to largest
const sizeUnits = ['B', 'KB', 'MB', 'GB', 'TB'];

// formatSize returns a size in bytes, in its largest units, of 1024, with
// one decimal place, when it isn't bytes, such as 1.5 MB
export function formatSize(bytes?: number): string {
    if (bytes === undefined || bytes < 0) {
        return '';
    }
    let size = bytes;
    let unit = 0;
    while (size >= 1024 && unit < sizeUnits.length - 1) {
        size /= 1024;
        unit++;
    }
    return unit === 0 ? `${size} B` : `${size.toFixed(1)} ${sizeUnits[unit]}`;
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


// The names of the days of the week, from Sunday, which is 0, or 7
const weekdays = ['Sundays', 'Mondays', 'Tuesdays', 'Wednesdays', 'Thursdays', 'Fridays', 'Saturdays'];

// describeSchedule returns a schedule, which is a cron expression, of
// minute, hour, day, month and weekday, in words, such as Weekdays at 08:00
// for 0 8 * * 1-5, or Every 15 minutes for */15 * * * *, or 0,15,30,45 * * *
// *, as the server stores it, or the expression, when it isn't one of those
// which are described
export function describeSchedule(schedule: string): string {
    const fields = schedule.trim().split(/\s+/);
    if (fields.length !== 5) {
        return schedule;
    }
    const [minute, hour, day, month, weekday] = fields;
    const number = (field: string, max: number) => /^\d+$/.test(field) && Number(field) <= max ? Number(field) : undefined;

    // Every so many minutes, or hours, every day
    if (day === '*' && month === '*' && weekday === '*') {
        const minutes = interval(minute, 60);
        if (minutes !== undefined && hour === '*') {
            return minutes === 1 ? 'Every minute' : `Every ${minutes} minutes`;
        }
        const hours = interval(hour, 24);
        const m = number(minute, 59);
        if (hours !== undefined && m !== undefined) {
            const every = hours === 1 ? 'Every hour' : `Every ${hours} hours`;
            return m === 0 ? every : `${every}, at ${m} minutes past`;
        }
    }

    // At a time, on the days which are described
    const h = number(hour, 23);
    const m = number(minute, 59);
    if (h === undefined || m === undefined || month !== '*') {
        return schedule;
    }
    const at = `at ${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`;
    if (day !== '*') {
        const d = number(day, 31);
        return d !== undefined && weekday === '*' ? `On day ${d} of each month ${at}` : schedule;
    }
    const days = weekdayList(weekday);
    if (days === undefined) {
        return schedule;
    } else if (days.length === 7) {
        return `Every day ${at}`;
    } else if (days.join() === '1,2,3,4,5') {
        return `Weekdays ${at}`;
    } else if (days.join() === '0,6') {
        return `Weekends ${at}`;
    }
    const names = days.map((d) => weekdays[d]);
    const list = names.length === 1 ? names[0] : `${names.slice(0, -1).join(', ')} and ${names[names.length - 1]}`;
    return `${list} ${at}`;
}

// interval returns how many apart the values of the field are, of those
// from 0 to the size, such as 15, for */15, or 0,15,30,45, or 1, for *, or
// undefined, when they aren't evenly apart, from 0, through the size
function interval(field: string, size: number): number | undefined {
    if (field === '*') {
        return 1;
    }
    const step = /^\*\/(\d+)$/.exec(field);
    if (step) {
        const n = Number(step[1]);
        return n > 0 && size % n === 0 ? n : undefined;
    }
    const values = field.split(',').map((value) => (/^\d+$/.test(value) ? Number(value) : NaN));
    if (values.length < 2 || values.some(Number.isNaN) || values[0] !== 0) {
        return undefined;
    }
    const n = values[1];
    if (n <= 0 || size % n !== 0 || values.length !== size / n || values.some((value, i) => value !== i * n)) {
        return undefined;
    }
    return n;
}

// weekdayList returns the days of the week of the field, from Sunday, which
// is 0, or 7, such as 1-5, or 1,3,5, or *, or undefined, when it isn't one
function weekdayList(field: string): number[] | undefined {
    if (field === '*') {
        return [0, 1, 2, 3, 4, 5, 6];
    }
    const days = new Set<number>();
    for (const part of field.split(',')) {
        const range = /^(\d)(?:-(\d))?$/.exec(part);
        if (!range) {
            return undefined;
        }
        const from = Number(range[1]);
        const to = range[2] === undefined ? from : Number(range[2]);
        if (from > 7 || to > 7 || to < from) {
            return undefined;
        }
        for (let d = from; d <= to; d++) {
            days.add(d % 7);
        }
    }
    return [...days].sort((a, b) => a - b);
}
