// Local release (no GitHub Actions): bumps the version, rotates the CHANGELOG,
// runs the quality gates, builds artifacts and publishes a GitHub release.
// Usage: bun scripts/release.ts <version> [--dry-run] [--skip-desktop]
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { basename, join } from "node:path";

export const repoUrl = "https://github.com/AlexGalhardo/vukem-kanban";

export const goTargets: readonly (readonly [string, string])[] = [
	["windows", "amd64"],
	["linux", "amd64"],
	["linux", "arm64"],
	["darwin", "amd64"],
	["darwin", "arm64"],
];

const preKinds = ["alpha", "beta", "rc"] as const;
type PreKind = (typeof preKinds)[number];

export interface Semver {
	major: number;
	minor: number;
	patch: number;
	pre?: { kind: PreKind; n: number };
}

const semverRe = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-(alpha|beta|rc)\.(0|[1-9]\d*))?$/;

export function parseSemver(v: string): Semver | undefined {
	const m = semverRe.exec(v);
	if (!m) return undefined;
	const sv: Semver = { major: Number(m[1]), minor: Number(m[2]), patch: Number(m[3]) };
	if (m[4]) sv.pre = { kind: m[4] as PreKind, n: Number(m[5]) };
	return sv;
}

export function compareSemver(a: string, b: string): number {
	const x = parseSemver(a);
	const y = parseSemver(b);
	if (!x || !y) throw new Error(`invalid semver: ${!x ? a : b}`);
	const core = x.major - y.major || x.minor - y.minor || x.patch - y.patch;
	if (core !== 0) return Math.sign(core);
	if (!x.pre && !y.pre) return 0;
	if (!x.pre) return 1;
	if (!y.pre) return -1;
	return Math.sign(preKinds.indexOf(x.pre.kind) - preKinds.indexOf(y.pre.kind) || x.pre.n - y.pre.n);
}

export function isPrerelease(v: string): boolean {
	return parseSemver(v)?.pre !== undefined;
}

const unreleasedHeading = /^## \[Unreleased\][^\n]*\n/m;
const sectionEnd = /^(## |\[[^\]]+\]: )/m;
const versionHeading = /^## \[(\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)\]/m;

/** Returns the body of `## [Unreleased]` (between its heading and the next section or link block). */
export function unreleasedBody(changelog: string): string {
	const head = unreleasedHeading.exec(changelog);
	if (!head) throw new Error("CHANGELOG.md has no `## [Unreleased]` section");
	const rest = changelog.slice(head.index + head[0].length);
	const end = sectionEnd.exec(rest);
	return rest.slice(0, end ? end.index : rest.length).trim();
}

/** Latest released version heading in the changelog, if any. */
export function latestReleased(changelog: string): string | undefined {
	return versionHeading.exec(changelog)?.[1];
}

/** Body of the `## [version]` section, without its heading. */
export function extractSection(changelog: string, version: string): string {
	const esc = version.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
	const head = new RegExp(`^## \\[${esc}\\][^\\n]*\\n`, "m").exec(changelog);
	if (!head) throw new Error(`CHANGELOG.md has no section for ${version}`);
	const rest = changelog.slice(head.index + head[0].length);
	const end = sectionEnd.exec(rest);
	return rest.slice(0, end ? end.index : rest.length).trim();
}

export function compareLinks(version: string, prev: string | undefined, repo: string = repoUrl): [string, string] {
	return [
		`[Unreleased]: ${repo}/compare/v${version}...HEAD`,
		prev ? `[${version}]: ${repo}/compare/v${prev}...v${version}` : `[${version}]: ${repo}/releases/tag/v${version}`,
	];
}

/** Rewrites the link reference block: new `[Unreleased]` and `[version]` lines first, older version links kept below. */
export function updateLinks(changelog: string, version: string, prev: string | undefined, repo: string = repoUrl): string {
	const lines = changelog.replace(/\s+$/, "").split("\n");
	const isLink = (l: string): boolean => /^\[[^\]]+\]: \S+$/.test(l);
	let start = lines.length;
	while (start > 0 && isLink(lines[start - 1] ?? "")) start--;
	const esc = version.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
	const stale = new RegExp(`^\\[(Unreleased|${esc})\\]: `);
	const kept = lines.slice(start).filter((l) => !stale.test(l));
	const body = lines.slice(0, start);
	while (body.length > 0 && body[body.length - 1] === "") body.pop();
	return `${[...body, "", ...compareLinks(version, prev, repo), ...kept].join("\n")}\n`;
}

/** Moves `[Unreleased]` into `[version] - date`, leaves an empty `[Unreleased]` and refreshes the compare links. */
export function rotateChangelog(changelog: string, version: string, date: string, repo: string = repoUrl): string {
	const crlf = changelog.includes("\r\n");
	const text = crlf ? changelog.replace(/\r\n/g, "\n") : changelog;
	const body = unreleasedBody(text);
	if (body === "") throw new Error("CHANGELOG.md `## [Unreleased]` is empty; nothing to release");
	if (new RegExp(`^## \\[${version.replace(/\./g, "\\.")}\\]`, "m").test(text)) {
		throw new Error(`CHANGELOG.md already has a section for ${version}`);
	}
	const prev = latestReleased(text);
	const head = unreleasedHeading.exec(text);
	if (!head) throw new Error("CHANGELOG.md has no `## [Unreleased]` section");
	const before = text.slice(0, head.index);
	const rest = text.slice(head.index + head[0].length);
	const end = sectionEnd.exec(rest);
	const after = end ? rest.slice(end.index) : "";
	const rotated = `${before}## [Unreleased]\n\n## [${version}] - ${date}\n\n${body}\n\n${after}`;
	const out = updateLinks(rotated, version, prev, repo);
	return crlf ? out.replace(/\n/g, "\r\n") : out;
}

export function setPackageVersion(pkg: string, version: string): string {
	const re = /("version"\s*:\s*")[^"]*(")/;
	if (!re.test(pkg)) throw new Error('package.json has no "version" field');
	return pkg.replace(re, `$1${version}$2`);
}

export function sha256sums(files: readonly string[]): string {
	return files
		.map((f) => `${createHash("sha256").update(readFileSync(f)).digest("hex")}  ${basename(f)}\n`)
		.join("");
}

export function releaseNotes(section: string, artifacts: readonly string[]): string {
	const list = artifacts.map((a) => `- \`${basename(a)}\``).join("\n");
	return `${section}\n\n## Downloads\n\n${list}\n\nVerifique a integridade com \`SHA256SUMS\` (\`sha256sum -c SHA256SUMS\`).\n`;
}

export function today(d: Date = new Date()): string {
	const pad = (n: number): string => String(n).padStart(2, "0");
	return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

interface Options {
	version: string;
	dryRun: boolean;
	skipDesktop: boolean;
}

export function parseArgs(argv: readonly string[]): Options {
	const opts: Options = { version: "", dryRun: false, skipDesktop: false };
	for (const a of argv) {
		if (a === "--dry-run") opts.dryRun = true;
		else if (a === "--skip-desktop") opts.skipDesktop = true;
		else if (a.startsWith("-")) throw new Error(`unknown flag ${a}`);
		else if (opts.version) throw new Error(`unexpected argument ${a}`);
		else opts.version = a.replace(/^v/, "");
	}
	if (!opts.version) throw new Error("usage: bun scripts/release.ts <version> [--dry-run] [--skip-desktop]");
	if (!parseSemver(opts.version)) {
		throw new Error(`invalid version ${opts.version}: want X.Y.Z or X.Y.Z-(alpha|beta|rc).N`);
	}
	return opts;
}

function run(cmd: string[]): void {
	console.log(`$ ${cmd.join(" ")}`);
	const p = Bun.spawnSync(cmd, { stdin: "inherit", stdout: "inherit", stderr: "inherit" });
	if (p.exitCode !== 0) throw new Error(`\`${cmd.join(" ")}\` failed with exit code ${p.exitCode}`);
}

function capture(cmd: string[]): { ok: boolean; out: string } {
	const p = Bun.spawnSync(cmd, { stdout: "pipe", stderr: "pipe" });
	return { ok: p.exitCode === 0, out: p.stdout.toString().trim() };
}

function step(msg: string): void {
	console.log(`\n==> ${msg}`);
}

function collectDesktop(version: string): string[] {
	const dir = "dist/desktop";
	if (!existsSync(dir)) return [];
	// Top-level installers only; win-unpacked/ and stale installers of other versions are ignored.
	return readdirSync(dir)
		.filter((f) => f.toLowerCase().endsWith(".exe") && f.includes(version))
		.map((f) => join(dir, f))
		.filter((f) => statSync(f).isFile());
}

async function release(opts: Options): Promise<void> {
	const { version, dryRun } = opts;
	const tag = `v${version}`;
	const mode = dryRun ? " (dry run)" : "";

	step(`preflight for ${tag}${mode}`);
	if (capture(["git", "status", "--porcelain"]).out !== "") throw new Error("git working tree is not clean");
	const branch = capture(["git", "rev-parse", "--abbrev-ref", "HEAD"]).out;
	if (branch !== "main") {
		if (!dryRun) throw new Error(`current branch is ${branch}; releases are cut from main`);
		console.warn(`warning: on branch ${branch}, a real release requires main`);
	}
	const current = readFileSync("VERSION", "utf8").trim();
	if (capture(["git", "rev-parse", "-q", "--verify", `refs/tags/${tag}`]).ok) throw new Error(`tag ${tag} already exists locally`);
	const remote = capture(["git", "ls-remote", "--tags", "origin", `refs/tags/${tag}`]);
	if (!remote.ok) {
		if (!dryRun) throw new Error("could not query tags on origin");
		console.warn("warning: could not query tags on origin");
	} else if (remote.out !== "") throw new Error(`tag ${tag} already exists on origin`);
	const cmp = compareSemver(version, current);
	// Equal is only valid for the very first release, whose version is already in VERSION.
	const currentTagged = capture(["git", "rev-parse", "-q", "--verify", `refs/tags/v${current}`]).ok;
	if (cmp < 0 || (cmp === 0 && currentTagged)) {
		throw new Error(`version ${version} must be greater than current ${current}`);
	}
	console.log(`current ${current} -> ${version}, branch ${branch}`);

	const tracked = ["VERSION", "package.json", "CHANGELOG.md"] as const;
	const originals = new Map<string, string>(tracked.map((f) => [f, readFileSync(f, "utf8")]));
	const restore = (): void => {
		for (const [f, content] of originals) writeFileSync(f, content);
	};
	let committed = false;
	try {
		step("bump VERSION, package.json and CHANGELOG.md");
		const changelog = rotateChangelog(originals.get("CHANGELOG.md") ?? "", version, today());
		writeFileSync("VERSION", `${version}\n`);
		writeFileSync("package.json", setPackageVersion(originals.get("package.json") ?? "", version));
		writeFileSync("CHANGELOG.md", changelog);
		const section = extractSection(changelog, version);

		step("quality gates");
		run(["golangci-lint", "run", "./..."]);
		run(["go", "test", "-race", "./..."]);
		run(["bun", "run", "smoke"]);

		step("build artifacts");
		rmSync("dist/go", { recursive: true, force: true });
		rmSync("dist/SHA256SUMS", { force: true });
		rmSync("dist/RELEASE_NOTES.md", { force: true });
		// Imported late: build-go.ts reads VERSION at module load, after the bump above.
		const { buildGo } = await import("./build-go.ts");
		const artifacts = goTargets.map(([goos, goarch]) => buildGo(goos, goarch));
		if (opts.skipDesktop) {
			console.log("desktop installer skipped (--skip-desktop)");
		} else if (!existsSync("desktop/electron-builder.yml")) {
			console.warn("warning: desktop/electron-builder.yml not found; skipping desktop installer");
		} else {
			run(["bun", "run", "dist"]);
			const installers = collectDesktop(version);
			if (installers.length === 0) throw new Error(`no installer for ${version} found in dist/desktop`);
			artifacts.push(...installers);
		}

		mkdirSync("dist", { recursive: true });
		writeFileSync("dist/SHA256SUMS", sha256sums(artifacts));
		writeFileSync("dist/RELEASE_NOTES.md", releaseNotes(section, artifacts));
		console.log(`wrote dist/SHA256SUMS and dist/RELEASE_NOTES.md (${artifacts.length} artifacts)`);

		const ghCmd = ["gh", "release", "create", tag, "--title", tag, "--notes-file", "dist/RELEASE_NOTES.md"];
		if (isPrerelease(version)) ghCmd.push("--prerelease");
		ghCmd.push(...artifacts, "dist/SHA256SUMS");
		const publish = [
			["git", "add", ...tracked],
			["git", "commit", "-m", `chore(release): ${tag}`],
			["git", "tag", "-a", tag, "-m", tag],
			["git", "push", "origin", "main", "--follow-tags"],
			ghCmd,
		];

		if (dryRun) {
			step("dry run: would run");
			for (const c of publish) console.log(`  ${c.map((a) => (a.includes(" ") ? JSON.stringify(a) : a)).join(" ")}`);
			console.log("\n--- release notes ---\n" + readFileSync("dist/RELEASE_NOTES.md", "utf8"));
			return;
		}

		step("commit, tag, push and publish");
		const [add, commit, ...rest] = publish;
		if (add) run(add);
		if (commit) run(commit);
		committed = true;
		for (const c of rest) run(c);
		console.log(`\nreleased ${tag}: ${repoUrl}/releases/tag/${tag}`);
	} finally {
		if (!committed) {
			restore();
			if (dryRun) console.log("restored VERSION, package.json and CHANGELOG.md");
			else console.warn("release aborted before commit; restored VERSION, package.json and CHANGELOG.md");
		}
	}
}

if (import.meta.main) {
	try {
		await release(parseArgs(process.argv.slice(2)));
	} catch (err) {
		console.error(`\nrelease failed: ${err instanceof Error ? err.message : String(err)}`);
		process.exit(1);
	}
}
