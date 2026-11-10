// The icons of the toolkit's own namespaces, such as memory's brain, which
// the Tools page's tiles show, as connectors have their logos: Carbon's,
// where it has one, and otherwise Tabler's, drawn in the text's colour, as
// Carbon's are, or a service's logo, such as Home Assistant's
//
// Tabler Icons (https://tabler.io/icons), whose spy, brain and news icons
// are here, are MIT licensed: Copyright (c) 2020-2024 Paweł Kuna.

import { svg } from 'lit';
import iconLoader from '@carbon/web-components/es/globals/internal/icon-loader.js';
import AI16 from '@carbon/icons/es/AI/16.js';
import Chat16 from '@carbon/icons/es/chat/16.js';
import Favorite16 from '@carbon/icons/es/favorite/16.js';
import Sun16 from '@carbon/icons/es/sun/16.js';

// tabler returns a Tabler icon, of the paths, as Carbon's are drawn, at 16
// pixels, in the text's colour
function tabler(paths: string[]) {
    return svg`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths.map((d) => svg`<path d=${d}/>`)}</svg>`;
}

const spy = ['M3 11h18', 'M5 11v-4a3 3 0 0 1 3 -3h8a3 3 0 0 1 3 3v4', 'M7 17m-3 0a3 3 0 1 0 6 0a3 3 0 1 0 -6 0', 'M17 17m-3 0a3 3 0 1 0 6 0a3 3 0 1 0 -6 0', 'M10 17h4'];
const brain = ['M15.5 13a3.5 3.5 0 0 0 -3.5 3.5v1a3.5 3.5 0 0 0 7 0v-1.8', 'M8.5 13a3.5 3.5 0 0 1 3.5 3.5v1a3.5 3.5 0 0 1 -7 0v-1.8', 'M17.5 16a3.5 3.5 0 0 0 0 -7h-.5', 'M19 9.3v-2.8a3.5 3.5 0 0 0 -7 0', 'M6.5 16a3.5 3.5 0 0 1 0 -7h.5', 'M5 9.3v-2.8a3.5 3.5 0 0 1 7 0v10'];
const news = ['M16 6h3a1 1 0 0 1 1 1v11a2 2 0 0 1 -4 0v-13a1 1 0 0 0 -1 -1h-10a1 1 0 0 0 -1 1v12a3 3 0 0 0 3 3h11', 'M8 8l4 0', 'M8 12l4 0', 'M8 16l4 0'];

// namespaceIcon returns the icon of the toolkit's own namespace, by its
// name: an icon, or the URL of a service's logo, or undefined, when it has
// none
export function namespaceIcon(name: string): string | unknown {
    switch (name) {
    case 'agent':
        return tabler(spy);
    case 'heartbeat':
        return iconLoader(Favorite16);
    case 'deepseek':
        return 'icon/provider/deepseek.svg';
    case 'homeassistant':
        return 'icon/namespace/homeassistant.svg';
    case 'llm':
        return iconLoader(AI16);
    case 'memory':
        return tabler(brain);
    case 'messaging':
        return iconLoader(Chat16);
    case 'newsapi':
        return tabler(news);
    case 'ollama':
        return 'icon/provider/ollama.svg';
    case 'weatherapi':
        return iconLoader(Sun16);
    }
    return undefined;
}
