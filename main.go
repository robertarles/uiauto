package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/keybind"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/getlantern/systray"
)

type Config struct {
	General struct {
		AppSelectPrefix    string `toml:"app_select_prefix"`
		WindowManagePrefix string `toml:"window_manage_prefix"`
	} `toml:"general"`
	AppSelect    map[string]AppConfig `toml:"app_select"`
	WindowManage map[string]string    `toml:"window_manage"`
}

type AppConfig struct {
	Command     string `toml:"command"`
	ProcessName string `toml:"process_name"`
	WindowClass string `toml:"window_class"`
}

const defaultConfig = `[general]
app_select_prefix = "Control-Mod1"
window_manage_prefix = "Cmd-Mod4"

[app_select]
b = { command = "firefox", process_name = "firefox", window_class = "Firefox" }
t = { command = "kitty", process_name = "kitty", window_class = "kitty" }
f = { command = "dolphin", process_name = "dolphin", window_class = "dolphin" }
# Add more applications here, e.g.:
# c = { command = "chromium", process_name = "chromium", window_class = "Chromium" }

[window_manage]
center = "Super-Mod1-M"
`

func main() {
	go func() {
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

		// Bind the window management keys
		for action, keyCombo := range config.WindowManage {
			bindWindowManagementKey(X, config.General.WindowManagePrefix+"-"+keyCombo, action)
		}
		fmt.Printf("Keymaps set. Use %s-[key] to manage windows.\n", config.General.AppSelectPrefix)
		xevent.Main(X)
	}()

	systray.Run(onReady, onExit)
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

func bindWindowManagementKey(X *xgbutil.XUtil, keyCombo string, action string) {
	err := keybind.KeyPressFun(
		func(X *xgbutil.XUtil, e xevent.KeyPressEvent) {
			if action == "center" {
				centerWindow()
			}
		}).Connect(X, X.RootWin(), keyCombo, true)
	if err != nil {
		fmt.Printf("Error binding window management key: %v\n", err)
	}
}

func centerWindow() {
	// Get screen dimensions using xrandr
	output, err := exec.Command("xrandr").Output()
	if err != nil {
		fmt.Println("Error getting screen dimensions:", err)
		return
	}

	// Assume the first connected screen is the primary one (more sophisticated parsing can be added)
	lines := strings.Split(string(output), "\n")
	var screenWidth, screenHeight int
	for _, line := range lines {
		if strings.Contains(line, " connected") {
			fields := strings.Fields(line)
			resolution := strings.Split(fields[2], "+")[0]
			dimensions := strings.Split(resolution, "x")
			screenWidth = strToInt(dimensions[0])
			screenHeight = strToInt(dimensions[1])
			break
		}
	}

	if screenWidth == 0 || screenHeight == 0 {
		fmt.Println("Error parsing screen dimensions.")
		return
	}

	// Calculate 75% of screen dimensions
	targetWidth := int(float64(screenWidth) * 0.75)
	targetHeight := int(float64(screenHeight) * 0.75)

	// Calculate top-left corner to center the window
	x := (screenWidth - targetWidth) / 2
	y := (screenHeight - targetHeight) / 2

	// Use wmctrl to move and resize the active window
	err = exec.Command("wmctrl", "-r", ":ACTIVE:", "-e", fmt.Sprintf("0,%d,%d,%d,%d", x, y, targetWidth, targetHeight)).Run()
	if err != nil {
		fmt.Println("Error centering window:", err)
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
	cmd := exec.Command("wmctrl", "-x", "-a", windowClass)
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error focusing %s: %v\n", windowClass, err)
	}
}

func startApp(command string) {
	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], parts[1:]...)
	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error starting %s: %v\n", command, err)
	}
}

func onReady() {
	systray.SetTitle("uiauto")
	systray.SetTooltip("UI Automation Tool")

	mQuit := systray.AddMenuItem("Quit", "Quit the application")

	go func() {
		<-mQuit.ClickedCh
		systray.Quit()
	}()
}

func onExit() {
	// Clean up here if needed
}

func strToInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}
