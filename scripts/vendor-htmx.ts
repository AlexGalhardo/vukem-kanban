// Copies the pinned htmx build from node_modules into the embedded static
// folder. The app must work offline (desktop), so no CDN.
import { copyFileSync } from "node:fs";

const src = "node_modules/htmx.org/dist/htmx.min.js";
const dest = "internal/web/static/htmx.min.js";
copyFileSync(src, dest);
console.log(`vendored ${src} -> ${dest}`);
