package main

import (
	"os"
	"path/filepath"
	"runtime"
)

// appName is the per-application directory name used under the resolved base
// directories.
const appName = "sx"

// baseKind identifies which category of base directory is being resolved.
type baseKind int

const (
	baseConfig baseKind = iota // XDG_CONFIG_HOME / ~/.config / %APPDATA%
	baseData                   // XDG_DATA_HOME  / ~/.local/share / %APPDATA%
	baseState                  // XDG_STATE_HOME / ~/.local/state / %LOCALAPPDATA%
	baseCache                  // XDG_CACHE_HOME / ~/.cache / %LOCALAPPDATA%
)

// pathEnv captures the inputs needed to resolve a base directory. Pulling the
// environment in as explicit fields keeps resolveBase pure and table-testable
// without mutating process state.
type pathEnv struct {
	goos      string // runtime.GOOS value, e.g. "linux", "darwin", "windows"
	home      string // os.UserHomeDir() result ("" if unavailable)
	xdgConfig string // XDG_CONFIG_HOME
	xdgData   string // XDG_DATA_HOME
	xdgState  string // XDG_STATE_HOME
	xdgCache  string // XDG_CACHE_HOME
	appData   string // %APPDATA%  (Windows roaming)
	localData string // %LOCALAPPDATA% (Windows local)
}

// currentPathEnv reads the live environment into a pathEnv.
func currentPathEnv() pathEnv {
	home, _ := os.UserHomeDir()
	return pathEnv{
		goos:      runtime.GOOS,
		home:      home,
		xdgConfig: os.Getenv("XDG_CONFIG_HOME"),
		xdgData:   os.Getenv("XDG_DATA_HOME"),
		xdgState:  os.Getenv("XDG_STATE_HOME"),
		xdgCache:  os.Getenv("XDG_CACHE_HOME"),
		appData:   os.Getenv("APPDATA"),
		localData: os.Getenv("LOCALAPPDATA"),
	}
}

// xdgFor returns the XDG_* override for the given base kind.
func (e pathEnv) xdgFor(kind baseKind) string {
	switch kind {
	case baseConfig:
		return e.xdgConfig
	case baseData:
		return e.xdgData
	case baseState:
		return e.xdgState
	case baseCache:
		return e.xdgCache
	}
	return ""
}

// resolveBase implements "option B" base-directory resolution:
//
//  1. An explicit, absolute XDG_* env var wins on ANY OS.
//  2. Otherwise on non-Windows (Unix incl. macOS): ~/.config, ~/.local/share,
//     ~/.local/state, ~/.cache.
//  3. Otherwise on Windows: %APPDATA% for config/data, %LOCALAPPDATA% for
//     state/cache.
//
// It returns the base directory (without the app name joined). An empty string
// is returned only when no candidate could be resolved (e.g. home unavailable
// and no env override).
func resolveBase(kind baseKind, e pathEnv) string {
	// 1. Explicit, absolute XDG override wins on any OS.
	if xdg := e.xdgFor(kind); xdg != "" && filepath.IsAbs(xdg) {
		return xdg
	}

	if e.goos == "windows" {
		// 3. Windows: %APPDATA% for config/data, %LOCALAPPDATA% for state/cache.
		switch kind {
		case baseConfig, baseData:
			if e.appData != "" {
				return e.appData
			}
		case baseState, baseCache:
			if e.localData != "" {
				return e.localData
			}
		}
		// Fall through to home-based defaults if the Windows vars are unset.
	}

	// 2. Unix (incl. macOS), and Windows fallback when its env vars are unset.
	if e.home == "" {
		return ""
	}
	switch kind {
	case baseConfig:
		return filepath.Join(e.home, ".config")
	case baseData:
		return filepath.Join(e.home, ".local", "share")
	case baseState:
		return filepath.Join(e.home, ".local", "state")
	case baseCache:
		return filepath.Join(e.home, ".cache")
	}
	return ""
}

// appDir resolves the per-app directory for the given base kind using the live
// environment. Returns "" when the base could not be resolved.
func appDir(kind baseKind) string {
	base := resolveBase(kind, currentPathEnv())
	if base == "" {
		return ""
	}
	return filepath.Join(base, appName)
}
