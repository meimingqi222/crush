package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginUninstallCmd)
	rootCmd.AddCommand(pluginCmd)
}

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage Crush plugins",
	Long:  "Install, list, and manage Crush plugins for extended functionality",
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install [plugin-name|path]",
	Short: "Install a plugin",
	Long: `Install a plugin from a local directory or remote registry.

Plugins are installed to .crush/plugins/ directory in your project.
Dependency installation uses pnpm by default to save disk space.
If pnpm is not available, falls back to npm.

Examples:
  crush plugin install morph-compact          # Install from registry
  crush plugin install ./my-plugin            # Install from local directory
  crush plugin install https://example.com/plugin.zip  # Install from URL

Tip: Install pnpm for better disk usage: npm install -g pnpm`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		workingDir, _ := cmd.Flags().GetString("cwd")
		if workingDir == "" {
			var err error
			workingDir, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		return installPlugin(source, workingDir)
	},
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		workingDir, _ := cmd.Flags().GetString("cwd")
		if workingDir == "" {
			var err error
			workingDir, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		return listPlugins(workingDir)
	},
}

var pluginUninstallCmd = &cobra.Command{
	Use:   "uninstall [plugin-name]",
	Short: "Uninstall a plugin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pluginName := args[0]
		workingDir, _ := cmd.Flags().GetString("cwd")
		if workingDir == "" {
			var err error
			workingDir, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		return uninstallPlugin(pluginName, workingDir)
	},
}

func installPlugin(source, workingDir string) error {
	// Create plugins directory if it doesn't exist
	pluginsDir := filepath.Join(workingDir, ".crush", "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// Check if source is a local directory
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		return installFromLocalDir(source, pluginsDir)
	}

	// Check if source is a URL
	if isURL(source) {
		return installFromURL(source, pluginsDir)
	}

	// Assume it's a plugin name from registry
	return installFromRegistry(source, pluginsDir)
}

func installFromLocalDir(source, pluginsDir string) error {
	// Read plugin metadata from source directory
	metadataPath := filepath.Join(source, "plugin.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return fmt.Errorf("plugin metadata not found: %s", metadataPath)
	}

	var metadata struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to read plugin metadata: %w", err)
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("invalid plugin metadata: %w", err)
	}

	// Copy plugin to plugins directory (excluding node_modules)
	pluginDir := filepath.Join(pluginsDir, metadata.Name)
	if err := copyPluginDir(source, pluginDir); err != nil {
		return fmt.Errorf("failed to copy plugin: %w", err)
	}

	// Install dependencies if package.json exists
	packageJSONPath := filepath.Join(pluginDir, "package.json")
	if _, err := os.Stat(packageJSONPath); err == nil {
		if err := installNPMDependencies(pluginDir); err != nil {
			return fmt.Errorf("failed to install dependencies: %w", err)
		}
	}

	fmt.Printf("Plugin %s installed successfully\n", metadata.Name)
	return nil
}

func installFromURL(url, pluginsDir string) error {
	// TODO: Implement URL download and installation
	_ = url
	_ = pluginsDir
	return fmt.Errorf("URL installation not yet implemented")
}

func installFromRegistry(name, pluginsDir string) error {
	// TODO: Implement registry installation
	// For now, support built-in plugins
	switch name {
	case "morph-compact":
		return installMorphCompact(pluginsDir)
	default:
		return fmt.Errorf("unknown plugin: %s", name)
	}
}

func installMorphCompact(pluginsDir string) error {
	// Create morph-compact plugin directory
	pluginDir := filepath.Join(pluginsDir, "morph-compact")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Create package.json
	packageJSON := `{
  "name": "crush-morph-compact-plugin",
  "private": true,
  "type": "module",
  "dependencies": {
    "@morphllm/morphsdk": "latest"
  }
}`
	if err := os.WriteFile(filepath.Join(pluginDir, "package.json"), []byte(packageJSON), 0o644); err != nil {
		return fmt.Errorf("failed to create package.json: %w", err)
	}

	// Create plugin.json with persistent mode for long-running process
	pluginJSON := `{
  "name": "morph-compact",
  "version": "1.0.0",
  "description": "Morph compact plugin for Crush",
  "command": "node",
  "args": ["index.mjs"],
  "mode": "persistent",
  "hooks": ["chat_messages_transform", "session_compacting"],
  "timeout_ms": 60000,
  "env": {
    "MORPH_API_KEY": "$MORPH_API_KEY",
    "MORPH_COMPACT_CONTEXT_THRESHOLD": "0.7",
    "MORPH_COMPACT_PRESERVE_RECENT": "2",
    "MORPH_COMPACT_RATIO": "0.3",
    "MORPH_MODEL_CONTEXT_TOKENS": "200000"
  }
}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pluginJSON), 0o644); err != nil {
		return fmt.Errorf("failed to create plugin.json: %w", err)
	}

	// Find the index.mjs file - try multiple locations
	sourcePath := findMorphCompactSource()
	if sourcePath == "" {
		return fmt.Errorf("could not find morph-compact plugin source (index.mjs)")
	}

	destPath := filepath.Join(pluginDir, "index.mjs")

	// Read source file
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source plugin: %w", err)
	}

	// Write to destination
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write plugin file: %w", err)
	}

	// Install dependencies
	if err := installNPMDependencies(pluginDir); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	fmt.Println("Morph compact plugin installed successfully")
	fmt.Println("Don't forget to set MORPH_API_KEY environment variable")
	return nil
}

// findMorphCompactSource locates the morph-compact index.mjs file.
// It checks multiple possible locations to handle different installation scenarios.
func findMorphCompactSource() string {
	// Try executable directory first (for installed binaries)
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "..", "examples", "plugins", "morph-compact", "index.mjs")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Try current working directory (for development)
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, "examples", "plugins", "morph-compact", "index.mjs")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Try relative to the source file location (for tests)
	_, filename, _, _ := runtime.Caller(0)
	if filename != "" {
		candidate := filepath.Join(filepath.Dir(filename), "..", "..", "..", "examples", "plugins", "morph-compact", "index.mjs")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

func listPlugins(workingDir string) error {
	pluginsDir := filepath.Join(workingDir, ".crush", "plugins")
	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		fmt.Println("No plugins installed")
		return nil
	}

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return fmt.Errorf("failed to read plugins directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No plugins installed")
		return nil
	}

	fmt.Println("Installed plugins:")
	for _, entry := range entries {
		if entry.IsDir() {
			pluginJSONPath := filepath.Join(pluginsDir, entry.Name(), "plugin.json")
			if _, err := os.Stat(pluginJSONPath); err == nil {
				data, err := os.ReadFile(pluginJSONPath)
				if err == nil {
					var metadata struct {
						Name        string `json:"name"`
						Version     string `json:"version"`
						Description string `json:"description"`
					}
					if err := json.Unmarshal(data, &metadata); err == nil {
						fmt.Printf("  %s (v%s): %s\n", metadata.Name, metadata.Version, metadata.Description)
						continue
					}
				}
			}
			fmt.Printf("  %s\n", entry.Name())
		}
	}

	return nil
}

func uninstallPlugin(name, workingDir string) error {
	pluginsDir := filepath.Join(workingDir, ".crush", "plugins")
	pluginDir := filepath.Join(pluginsDir, name)

	// Ensure the resolved path is within the plugins directory to prevent path traversal.
	cleanPluginDir := filepath.Clean(pluginDir) + string(filepath.Separator)
	cleanPluginsDir := filepath.Clean(pluginsDir) + string(filepath.Separator)
	if !strings.HasPrefix(cleanPluginDir, cleanPluginsDir) {
		return fmt.Errorf("invalid plugin name: %s", name)
	}

	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		return fmt.Errorf("plugin not found: %s", name)
	}

	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("failed to remove plugin: %w", err)
	}

	fmt.Printf("Plugin %s uninstalled successfully\n", name)
	return nil
}

func installNPMDependencies(dir string) error {
	// Check if node_modules already exists
	if _, err := os.Stat(filepath.Join(dir, "node_modules")); err == nil {
		// node_modules already exists
		return nil
	}

	// Prefer pnpm to avoid duplicate node_modules and save disk space
	if _, err := exec.LookPath("pnpm"); err == nil {
		fmt.Printf("Installing dependencies with pnpm in %s...\n", dir)
		cmd := exec.Command("pnpm", "install")
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pnpm install failed: %w", err)
		}
		return nil
	}

	// Fallback to npm if pnpm is not available
	if _, err := exec.LookPath("npm"); err == nil {
		fmt.Printf("pnpm not found, falling back to npm in %s...\n", dir)
		fmt.Println("Tip: Install pnpm for better disk usage: npm install -g pnpm")
		cmd := exec.Command("npm", "install")
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("npm install failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("no package manager found. Please install pnpm (recommended) or npm")
}

// copyPluginDir copies a plugin directory, excluding node_modules to save space
// and avoid copying large dependency trees.
func copyPluginDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Skip node_modules directory
		if entry.IsDir() && entry.Name() == "node_modules" {
			continue
		}

		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyPluginDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyDir(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read source directory entries
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
