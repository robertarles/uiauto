package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/xevent"
)

type Config struct {
	General struct {
		AppSelectPrefix string `toml:"app_select_prefix"`
	} `toml:"general"`
	AppSelect map[string]AppConfig `toml:"app_select"`
}

type AppConfig struct {
	Command     string `toml:"command"`
	ProcessName string `toml:"process_name"`
	WindowClass string `toml:"window_class"`
}

const defaultConfig = `[general]
app_select_prefix = "Control-Mod1"

[app_select]
b = { command = "firefox", process_name = "firefox", window_class = "Firefox" }
t = { command = "kitty", process_name = "kitty", window_class = "kitty" }
# Add more applications here, e.g.:
# c = { command = "chromium", process_name = "chromium", window_class = "Chromium" }
`

func main() {
	config, err := loadOrCreateConfig()
	if err != nil {
		fmt.Println("Error loading or creating config:", err)
		return
	}

	X, err := xgbutil.NewConn()
	if err != nil {
		fmt.Println("Error connecting to X server:", err)
		return
	}

	keybind.Initialize(X)

	for key, app := range config.AppSelect {
		bindKey(X, config.General.AppSelectPrefix+"-"+key, app)
	}

	fmt.Printf("Keymaps set. Use %s-[key] to launch or focus applications.\n", config.General.AppSelectPrefix)
	xevent.Main(X)
}

func loadOrCreateConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(homeDir, ".config", "uiauto")
	configPath := filepath.Join(configDir, "uiauto.conf")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		err = os.MkdirAll(configDir, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create config directory: %v", err)
		}

		err = os.WriteFile(configPath, []byte(defaultConfig), 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create default config file: %v", err)
		}
		fmt.Println("Created default config file at", configPath)
	}

	var config Config
	_, err = toml.DecodeFile(configPath, &config)
	return &config, err
}

func bindKey(X *xgbutil.XUtil, keyCombo string, app AppConfig) {
	err := keybind.KeyPressFun(
		func(X *xgbutil.XUtil, e xevent.KeyPressEvent) {
			openOrFocusApp(app)
		}).Connect(X, X.RootWin(), keyCombo, true)
	if err != nil {
		fmt.Printf("Error binding key for %s: %v\n", app.Command, err)
	}
}

func openOrFocusApp(app AppConfig) {
	if isAppRunning(app.ProcessName) {
		focusApp(app.WindowClass)
	} else {
		startApp(app.Command)
	}
}

func isAppRunning(processName string) bool {
	cmd := exec.Command("pgrep", "-f", processName)
	output, err := cmd.Output()
	return err == nil && len(output) > 0
}

func focusApp(windowClass string) {
	fmt.Printf("Focusing %s...\n", windowClass)
	cmd := exec.Command("wmctrl", "-x", "-a", windowClass)
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error focusing %s: %v\n", windowClass, err)
	}
}

func startApp(command string) {
	fmt.Printf("Starting %s...\n", command)
	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], parts[1:]...)
	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error starting %s: %v\n", command, err)
	}
}
