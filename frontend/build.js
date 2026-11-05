import esbuild from 'esbuild';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';
import { cpSync, readFileSync } from 'fs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const buildDir = resolve(__dirname, '../build/frontend');

// carbonFonts strips the fonts which Carbon's prebuilt styles load from IBM's
// CDN, as the UI bundles its own (see src/fonts.css)
const carbonFonts = {
    name: 'carbon-fonts',
    setup(build) {
        build.onLoad({ filter: /@carbon[\\/]styles[\\/]css[\\/]styles(\.min)?\.css$/ }, (args) => ({
            contents: readFileSync(args.path, 'utf8').replace(/@font-face\s*\{[^}]*\}/g, ''),
            loader: 'css',
        }));
    },
};

// Build TypeScript → build/frontend/dist/, with the fonts its styles use in
// build/frontend/dist/fonts/
await esbuild.build({
    entryPoints: ['src/index.ts'],
    bundle: true,
    outdir: `${buildDir}/dist`,
    format: 'esm',
    target: 'es2021',
    minify: true,
    sourcemap: true,
    loader: { '.woff2': 'file' },
    assetNames: 'fonts/[name]-[hash]',
    plugins: [carbonFonts],
}).catch(() => process.exit(1));

// Copy source assets (index.html etc.) → build/frontend/dist/
cpSync(resolve(__dirname, 'assets/dist'), `${buildDir}/dist`, { recursive: true });

// Copy embed.go → build/frontend/embed.go, so the server embeds the UI
cpSync(resolve(__dirname, 'assets/embed.go'), `${buildDir}/embed.go`);

console.log(`Built → ${buildDir}`);
