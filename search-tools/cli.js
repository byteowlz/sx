#!/usr/bin/env node

/**
 * Browser Tools CLI
 * A cross-platform CLI for web search and content extraction
 */

import { spawn } from "child_process";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const VERSION = "1.0.0";

const HELP = `
browser-tools v${VERSION}

Cross-platform CLI for web search and content extraction using headless Chrome.

USAGE:
  bt <command> [options]

COMMANDS:
  search <query>      Search Google and return results
  content <url>       Extract readable content from a URL
  help                Show this help message

SEARCH OPTIONS:
  -n, --num <num>     Number of results (default: 5)
  -c, --content       Include page content for each result
  -s, --setup         Open browser to solve CAPTCHA
  --sync              Force re-sync Chrome profile

CONTENT OPTIONS:
  --sync              Force re-sync Chrome profile

EXAMPLES:
  bt search "rust programming"
  bt search "AI news" -n 10
  bt search "climate change" -c
  bt search "test" --setup
  bt content https://example.com
`;

function parseArgs(args) {
	const result = {
		command: null,
		query: [],
		options: {
			num: 5,
			content: false,
			setup: false,
			sync: false,
		},
	};

	let i = 0;
	while (i < args.length) {
		const arg = args[i];

		if (!result.command && !arg.startsWith("-")) {
			result.command = arg;
			i++;
			continue;
		}

		switch (arg) {
			case "-n":
			case "--num":
				result.options.num = parseInt(args[++i], 10) || 5;
				break;
			case "-c":
			case "--content":
				result.options.content = true;
				break;
			case "-s":
			case "--setup":
				result.options.setup = true;
				break;
			case "--sync":
				result.options.sync = true;
				break;
			case "-h":
			case "--help":
				result.command = "help";
				break;
			case "-v":
			case "--version":
				result.command = "version";
				break;
			default:
				if (!arg.startsWith("-")) {
					result.query.push(arg);
				}
		}
		i++;
	}

	return result;
}

function runScript(script, args) {
	const scriptPath = path.join(__dirname, script);
	const child = spawn("node", [scriptPath, ...args], {
		stdio: "inherit",
		cwd: __dirname,
	});

	child.on("close", (code) => {
		process.exit(code || 0);
	});

	child.on("error", (err) => {
		console.error(`Failed to run ${script}:`, err.message);
		process.exit(1);
	});
}

function main() {
	const args = process.argv.slice(2);

	if (args.length === 0) {
		console.log(HELP);
		process.exit(0);
	}

	const parsed = parseArgs(args);

	switch (parsed.command) {
		case "help":
		case undefined:
			console.log(HELP);
			break;

		case "version":
			console.log(`browser-tools v${VERSION}`);
			break;

		case "search":
		case "s": {
			const query = parsed.query.join(" ");
			if (!query) {
				console.error("Error: search requires a query");
				console.error("Usage: bt search <query> [options]");
				process.exit(1);
			}

			const scriptArgs = [query, "-n", String(parsed.options.num)];
			if (parsed.options.content) scriptArgs.push("--content");
			if (parsed.options.setup) scriptArgs.push("--setup");
			if (parsed.options.sync) scriptArgs.push("--sync");

			runScript("search.js", scriptArgs);
			break;
		}

		case "content":
		case "c": {
			const url = parsed.query[0];
			if (!url) {
				console.error("Error: content requires a URL");
				console.error("Usage: bt content <url> [options]");
				process.exit(1);
			}

			const scriptArgs = [url];
			if (parsed.options.sync) scriptArgs.push("--sync");

			runScript("content.js", scriptArgs);
			break;
		}

		default:
			// Treat unknown command as a search query
			const query = [parsed.command, ...parsed.query].join(" ");
			const scriptArgs = [query, "-n", String(parsed.options.num)];
			if (parsed.options.content) scriptArgs.push("--content");
			if (parsed.options.setup) scriptArgs.push("--setup");
			if (parsed.options.sync) scriptArgs.push("--sync");

			runScript("search.js", scriptArgs);
	}
}

main();
