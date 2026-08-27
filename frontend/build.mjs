import { copyFile, mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(fileURLToPath(import.meta.url));
const dist = join(root, "..", "internal", "api", "web", "dist");

await mkdir(dist, { recursive: true });
await copyFile(join(root, "src", "index.html"), join(dist, "index.html"));
await copyFile(join(root, "src", "styles.css"), join(dist, "styles.css"));

const ts = await readFile(join(root, "src", "app.ts"), "utf8");
await writeFile(join(dist, "app.js"), ts, "utf8");
