package connect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const serviceName = "com.mcphub.mcp-connect"

func ServicePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "LaunchAgents", serviceName+".plist"), nil
	case "linux":
		return filepath.Join(home, ".config", "systemd", "user", "mcp-connect.service"), nil
	default:
		return "", fmt.Errorf("service installation is unsupported on %s", runtime.GOOS)
	}
}

func InstallService(configPath, socket string) (string, error) {
	path, err := ServicePath()
	if err != nil {
		return "", err
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	var content string
	if runtime.GOOS == "darwin" {
		content = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array><string>%s</string><string>daemon</string><string>--config</string><string>%s</string><string>--socket</string><string>%s</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
</dict></plist>
`, serviceName, exe, configPath, socket)
	} else {
		content = fmt.Sprintf(`[Unit]
Description=MCPHub Connect daemon
After=network-online.target

[Service]
ExecStart=%s daemon --config %s --socket %s
Restart=on-failure
RestartSec=3

[Install]
WantedBy=default.target
`, exe, configPath, socket)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return "", err
	}
	return path, nil
}

func UninstallService() (string, error) {
	path, err := ServicePath()
	if err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return path, nil
}

func EnableService() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	path, err := ServicePath()
	if err != nil {
		return err
	}
	_ = path
	return exec.Command("systemctl", "--user", "enable", "--now", "mcp-connect.service").Run()
}
