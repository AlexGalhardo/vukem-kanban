import { describe, expect, test } from "bun:test";
import {
	compareSemver,
	extractSection,
	isPrerelease,
	latestReleased,
	parseArgs,
	parseSemver,
	releaseNotes,
	rotateChangelog,
	setPackageVersion,
	today,
	unreleasedBody,
	updateLinks,
} from "./release.ts";

const repo = "https://github.com/AlexGalhardo/trello-golang-htmx";

describe("semver", () => {
	test.each(["0.0.1", "1.2.3", "10.0.0-alpha.0", "1.0.0-beta.2", "1.0.0-rc.11"])("parses %s", (v) => {
		expect(parseSemver(v)).toBeDefined();
	});

	test.each(["1.2", "01.2.3", "1.2.3-rc", "1.2.3-rc.01", "1.2.3-dev.1", "v1.2.3", "1.2.3+build", ""])("rejects %s", (v) => {
		expect(parseSemver(v)).toBeUndefined();
	});

	test.each([
		["0.0.2", "0.0.1", 1],
		["0.1.0", "0.0.9", 1],
		["1.0.0", "0.99.99", 1],
		["1.0.0", "1.0.0", 0],
		["1.0.0-rc.1", "1.0.0", -1],
		["1.0.0", "1.0.0-rc.9", 1],
		["1.0.0-alpha.9", "1.0.0-beta.1", -1],
		["1.0.0-beta.1", "1.0.0-rc.1", -1],
		["1.0.0-rc.10", "1.0.0-rc.2", 1],
		["1.0.0-rc.2", "1.0.0-rc.2", 0],
		["1.0.1-alpha.1", "1.0.0", 1],
	] as const)("compare(%s, %s) = %d", (a, b, want) => {
		expect(compareSemver(a, b)).toBe(want);
	});

	test("compare throws on invalid input", () => {
		expect(() => compareSemver("1.0", "1.0.0")).toThrow();
	});

	test("prerelease detection", () => {
		expect(isPrerelease("1.0.0-rc.1")).toBe(true);
		expect(isPrerelease("0.1.0-alpha.3")).toBe(true);
		expect(isPrerelease("1.0.0")).toBe(false);
	});
});

const header = `# Changelog

All notable changes to this project will be documented in this file.

`;

const first = `${header}## [Unreleased]

### Added

- Item A.
- Item B.
`;

describe("changelog", () => {
	test("unreleased body stops at next section", () => {
		const text = `${header}## [Unreleased]\n\n### Fixed\n\n- X.\n\n## [0.0.1] - 2026-01-01\n\n- Old.\n`;
		expect(unreleasedBody(text)).toBe("### Fixed\n\n- X.");
	});

	test("first release uses releases/tag link", () => {
		const out = rotateChangelog(first, "0.0.1", "2026-10-05", repo);
		expect(out).toBe(`${header}## [Unreleased]

## [0.0.1] - 2026-10-05

### Added

- Item A.
- Item B.

[Unreleased]: ${repo}/compare/v0.0.1...HEAD
[0.0.1]: ${repo}/releases/tag/v0.0.1
`);
		expect(unreleasedBody(out)).toBe("");
		expect(extractSection(out, "0.0.1")).toBe("### Added\n\n- Item A.\n- Item B.");
	});

	test("subsequent release compares with previous and keeps older links", () => {
		const v1 = rotateChangelog(first, "0.0.1", "2026-10-05", repo);
		const withNew = v1.replace("## [Unreleased]\n", "## [Unreleased]\n\n### Fixed\n\n- Bug.\n");
		const out = rotateChangelog(withNew, "0.1.0-rc.1", "2026-11-01", repo);
		expect(latestReleased(out)).toBe("0.1.0-rc.1");
		expect(extractSection(out, "0.1.0-rc.1")).toBe("### Fixed\n\n- Bug.");
		expect(extractSection(out, "0.0.1")).toBe("### Added\n\n- Item A.\n- Item B.");
		expect(out.endsWith(`[Unreleased]: ${repo}/compare/v0.1.0-rc.1...HEAD
[0.1.0-rc.1]: ${repo}/compare/v0.0.1...v0.1.0-rc.1
[0.0.1]: ${repo}/releases/tag/v0.0.1
`)).toBe(true);
		expect(out.match(/^\[Unreleased\]:/gm)?.length).toBe(1);
		expect(out.indexOf("## [Unreleased]")).toBeLessThan(out.indexOf("## [0.1.0-rc.1]"));
		expect(out.indexOf("## [0.1.0-rc.1]")).toBeLessThan(out.indexOf("## [0.0.1]"));
	});

	test("fails on empty Unreleased", () => {
		const v1 = rotateChangelog(first, "0.0.1", "2026-10-05", repo);
		expect(() => rotateChangelog(v1, "0.0.2", "2026-10-06", repo)).toThrow(/empty/);
	});

	test("fails without Unreleased heading", () => {
		expect(() => rotateChangelog(header, "0.0.1", "2026-10-05", repo)).toThrow(/Unreleased/);
	});

	test("fails when the version section already exists", () => {
		const v1 = rotateChangelog(first, "0.0.1", "2026-10-05", repo);
		const again = v1.replace("## [Unreleased]\n", "## [Unreleased]\n\n- More.\n");
		expect(() => rotateChangelog(again, "0.0.1", "2026-10-05", repo)).toThrow(/already/);
	});

	test("preserves CRLF line endings", () => {
		const out = rotateChangelog(first.replace(/\n/g, "\r\n"), "0.0.1", "2026-10-05", repo);
		expect(out.replace(/\r\n/g, "")).not.toContain("\n");
		expect(out).toContain("## [0.0.1] - 2026-10-05\r\n");
	});

	test("updateLinks appends a link block when none exists", () => {
		expect(updateLinks("# C\n\n## [Unreleased]\n\n\n", "1.0.0", "0.9.0", repo)).toBe(
			`# C\n\n## [Unreleased]\n\n[Unreleased]: ${repo}/compare/v1.0.0...HEAD\n[1.0.0]: ${repo}/compare/v0.9.0...v1.0.0\n`,
		);
	});

	test("extractSection fails for unknown version", () => {
		expect(() => extractSection(first, "9.9.9")).toThrow();
	});
});

describe("misc helpers", () => {
	test("setPackageVersion keeps tab formatting", () => {
		const pkg = '{\n\t"name": "x",\n\t"version": "0.0.1",\n\t"dependencies": {\n\t\t"y": "1.0.0"\n\t}\n}\n';
		expect(setPackageVersion(pkg, "0.1.0")).toBe(pkg.replace('"version": "0.0.1"', '"version": "0.1.0"'));
		expect(() => setPackageVersion("{}", "1.0.0")).toThrow();
	});

	test("releaseNotes lists artifacts by basename", () => {
		const notes = releaseNotes("### Added\n\n- A.", ["dist/go/kanban-1.0.0-linux-amd64", "dist/desktop/Kanban Setup 1.0.0.exe"]);
		expect(notes.startsWith("### Added\n\n- A.\n\n## Downloads\n")).toBe(true);
		expect(notes).toContain("- `kanban-1.0.0-linux-amd64`");
		expect(notes).toContain("- `Kanban Setup 1.0.0.exe`");
		expect(notes).toContain("SHA256SUMS");
	});

	test("today formats local date", () => {
		expect(today(new Date(2026, 0, 5))).toBe("2026-01-05");
	});

	test("parseArgs", () => {
		expect(parseArgs(["v1.2.3", "--dry-run"])).toEqual({ version: "1.2.3", dryRun: true, skipDesktop: false });
		expect(parseArgs(["--skip-desktop", "1.0.0-rc.1"])).toEqual({ version: "1.0.0-rc.1", dryRun: false, skipDesktop: true });
		expect(() => parseArgs([])).toThrow(/usage/);
		expect(() => parseArgs(["1.2"])).toThrow(/invalid version/);
		expect(() => parseArgs(["1.2.3", "--force"])).toThrow(/unknown flag/);
		expect(() => parseArgs(["1.2.3", "1.2.4"])).toThrow(/unexpected/);
	});
});
