/**
 * Cross-platform utilities for Chrome profile and executable detection
 */

import os from "os";
import path from "path";
import fs from "fs";
import { execSync } from "child_process";

const platform = os.platform();
const homedir = os.homedir();

/**
 * Get the default Chrome user data directory for the current platform
 */
export function getChromeProfilePath() {
	switch (platform) {
		case "darwin":
			return path.join(homedir, "Library", "Application Support", "Google", "Chrome");
		case "win32":
			return path.join(process.env.LOCALAPPDATA || path.join(homedir, "AppData", "Local"), "Google", "Chrome", "User Data");
		case "linux":
			// Check common locations
			const linuxPaths = [
				path.join(homedir, ".config", "google-chrome"),
				path.join(homedir, ".config", "chromium"),
				path.join(homedir, "snap", "chromium", "common", "chromium"),
			];
			for (const p of linuxPaths) {
				if (fs.existsSync(p)) return p;
			}
			return linuxPaths[0]; // Default to google-chrome
		default:
			throw new Error(`Unsupported platform: ${platform}`);
	}
}

/**
 * Get the Chrome executable path for the current platform
 */
export function getChromeExecutablePath() {
	switch (platform) {
		case "darwin": {
			const paths = [
				"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
				"/Applications/Chromium.app/Contents/MacOS/Chromium",
				path.join(homedir, "Applications", "Google Chrome.app", "Contents", "MacOS", "Google Chrome"),
			];
			for (const p of paths) {
				if (fs.existsSync(p)) return p;
			}
			return paths[0];
		}
		case "win32": {
			const paths = [
				path.join(process.env.PROGRAMFILES || "C:\\Program Files", "Google", "Chrome", "Application", "chrome.exe"),
				path.join(process.env["PROGRAMFILES(X86)"] || "C:\\Program Files (x86)", "Google", "Chrome", "Application", "chrome.exe"),
				path.join(process.env.LOCALAPPDATA || "", "Google", "Chrome", "Application", "chrome.exe"),
			];
			for (const p of paths) {
				if (fs.existsSync(p)) return p;
			}
			return paths[0];
		}
		case "linux": {
			const paths = [
				"/usr/bin/google-chrome",
				"/usr/bin/google-chrome-stable",
				"/usr/bin/chromium",
				"/usr/bin/chromium-browser",
				"/snap/bin/chromium",
			];
			for (const p of paths) {
				if (fs.existsSync(p)) return p;
			}
			// Try which command as fallback
			try {
				const result = execSync("which google-chrome || which chromium || which chromium-browser", { encoding: "utf8" }).trim();
				if (result) return result;
			} catch {}
			return paths[0];
		}
		default:
			throw new Error(`Unsupported platform: ${platform}`);
	}
}

/**
 * Directories/files to exclude when copying Chrome profile
 */
const PROFILE_EXCLUDES = [
	"SingletonLock",
	"SingletonSocket",
	"SingletonCookie",
	"lockfile",
	"Lock",
	"BrowserMetrics",
	"BrowserMetrics-spare.pma",
	"Crashpad",
	"Cache",
	"Code Cache",
	"GPUCache",
	"GrShaderCache",
	"ShaderCache",
	"Service Worker",
	"CacheStorage",
	"blob_storage",
	"Session Storage",
	"Local Storage",
];

/**
 * Check if a path should be excluded from copy
 */
function shouldExclude(name) {
	return PROFILE_EXCLUDES.some(excl => 
		name === excl || 
		name.startsWith(excl) || 
		name.endsWith(".lock") || 
		name.endsWith(".tmp")
	);
}

/**
 * Recursively copy directory with exclusions (cross-platform)
 */
function copyDirSync(src, dest) {
	if (!fs.existsSync(src)) return;
	
	if (!fs.existsSync(dest)) {
		fs.mkdirSync(dest, { recursive: true });
	}

	const entries = fs.readdirSync(src, { withFileTypes: true });
	
	for (const entry of entries) {
		if (shouldExclude(entry.name)) continue;
		
		const srcPath = path.join(src, entry.name);
		const destPath = path.join(dest, entry.name);
		
		try {
			if (entry.isDirectory()) {
				copyDirSync(srcPath, destPath);
			} else if (entry.isFile()) {
				// Only copy if source is newer or dest doesn't exist
				const srcStat = fs.statSync(srcPath);
				let shouldCopy = true;
				
				if (fs.existsSync(destPath)) {
					const destStat = fs.statSync(destPath);
					shouldCopy = srcStat.mtimeMs > destStat.mtimeMs;
				}
				
				if (shouldCopy) {
					fs.copyFileSync(srcPath, destPath);
				}
			}
		} catch (e) {
			// Skip files that can't be copied (permissions, locks, etc.)
		}
	}
}

/**
 * Sync Chrome profile to cache directory (cross-platform)
 */
export function syncChromeProfile(sourceProfile, cacheProfile) {
	// On Unix-like systems, try rsync first (faster for incremental updates)
	if (platform !== "win32") {
		try {
			const excludeFlags = PROFILE_EXCLUDES.map(e => `--exclude='${e}*'`).join(" ");
			execSync(
				`rsync -a --delete ${excludeFlags} "${sourceProfile}/" "${cacheProfile}/"`,
				{ stdio: "pipe" }
			);
			return;
		} catch {
			// rsync not available, fall through to JS implementation
		}
	}
	
	// Cross-platform JavaScript fallback
	copyDirSync(sourceProfile, cacheProfile);
}

/**
 * Check if Chrome is available
 */
export function isChromeAvailable() {
	const execPath = getChromeExecutablePath();
	return fs.existsSync(execPath);
}

export { platform };
