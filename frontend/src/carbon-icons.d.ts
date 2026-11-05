// Carbon's icons are descriptors of their SVG, which Carbon's iconLoader
// renders, and have no type declarations
declare module '@carbon/icons/es/*' {
    import type { CarbonIconDescriptor } from '@carbon/web-components/es/globals/internal/icon-loader-utils.js';
    const icon: CarbonIconDescriptor;
    export default icon;
}
