package main

import (
	"path/filepath"
	"testing"
)

func TestResolveBase(t *testing.T) {
	const (
		unixHome = "/home/user"
		winHome  = `C:\Users\user`
		appData  = `C:\Users\user\AppData\Roaming`
		localApp = `C:\Users\user\AppData\Local`
	)

	tests := []struct {
		name string
		kind baseKind
		env  pathEnv
		want string
	}{
		// --- Unix (linux) defaults ---
		{
			name: "linux config default",
			kind: baseConfig,
			env:  pathEnv{goos: "linux", home: unixHome},
			want: filepath.Join(unixHome, ".config"),
		},
		{
			name: "linux data default",
			kind: baseData,
			env:  pathEnv{goos: "linux", home: unixHome},
			want: filepath.Join(unixHome, ".local", "share"),
		},
		{
			name: "linux state default",
			kind: baseState,
			env:  pathEnv{goos: "linux", home: unixHome},
			want: filepath.Join(unixHome, ".local", "state"),
		},
		{
			name: "linux cache default",
			kind: baseCache,
			env:  pathEnv{goos: "linux", home: unixHome},
			want: filepath.Join(unixHome, ".cache"),
		},

		// --- macOS uses the unix defaults too (option B) ---
		{
			name: "darwin config uses unix default",
			kind: baseConfig,
			env:  pathEnv{goos: "darwin", home: unixHome},
			want: filepath.Join(unixHome, ".config"),
		},
		{
			name: "darwin state uses unix default",
			kind: baseState,
			env:  pathEnv{goos: "darwin", home: unixHome},
			want: filepath.Join(unixHome, ".local", "state"),
		},

		// --- Explicit absolute XDG wins on any OS ---
		{
			name: "linux absolute XDG_CONFIG_HOME wins",
			kind: baseConfig,
			env:  pathEnv{goos: "linux", home: unixHome, xdgConfig: "/custom/cfg"},
			want: "/custom/cfg",
		},
		{
			name: "darwin absolute XDG_STATE_HOME wins",
			kind: baseState,
			env:  pathEnv{goos: "darwin", home: unixHome, xdgState: "/custom/state"},
			want: "/custom/state",
		},
		{
			// Use a forward-slash absolute path so filepath.IsAbs recognizes it
			// regardless of the host OS running the test. On a real Windows
			// build a path like `D:\xdgcfg` is also absolute.
			name: "windows absolute XDG_CONFIG_HOME wins over APPDATA",
			kind: baseConfig,
			env:  pathEnv{goos: "windows", home: winHome, appData: appData, xdgConfig: "/xdgcfg"},
			want: "/xdgcfg",
		},

		// --- Relative XDG is ignored (must be absolute) ---
		{
			name: "linux relative XDG_CONFIG_HOME ignored",
			kind: baseConfig,
			env:  pathEnv{goos: "linux", home: unixHome, xdgConfig: "relative/cfg"},
			want: filepath.Join(unixHome, ".config"),
		},

		// --- Windows: APPDATA for config/data, LOCALAPPDATA for state/cache ---
		{
			name: "windows config uses APPDATA",
			kind: baseConfig,
			env:  pathEnv{goos: "windows", home: winHome, appData: appData, localData: localApp},
			want: appData,
		},
		{
			name: "windows data uses APPDATA",
			kind: baseData,
			env:  pathEnv{goos: "windows", home: winHome, appData: appData, localData: localApp},
			want: appData,
		},
		{
			name: "windows state uses LOCALAPPDATA",
			kind: baseState,
			env:  pathEnv{goos: "windows", home: winHome, appData: appData, localData: localApp},
			want: localApp,
		},
		{
			name: "windows cache uses LOCALAPPDATA",
			kind: baseCache,
			env:  pathEnv{goos: "windows", home: winHome, appData: appData, localData: localApp},
			want: localApp,
		},

		// --- Windows fallback to home defaults when vars unset ---
		{
			name: "windows config falls back to home when APPDATA unset",
			kind: baseConfig,
			env:  pathEnv{goos: "windows", home: winHome},
			want: filepath.Join(winHome, ".config"),
		},
		{
			name: "windows state falls back to home when LOCALAPPDATA unset",
			kind: baseState,
			env:  pathEnv{goos: "windows", home: winHome},
			want: filepath.Join(winHome, ".local", "state"),
		},

		// --- No resolvable base ---
		{
			name: "unix empty home and no override yields empty",
			kind: baseConfig,
			env:  pathEnv{goos: "linux"},
			want: "",
		},
		{
			name: "windows empty home and no vars yields empty",
			kind: baseState,
			env:  pathEnv{goos: "windows"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveBase(tt.kind, tt.env)
			if got != tt.want {
				t.Errorf("resolveBase(%v) = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}
