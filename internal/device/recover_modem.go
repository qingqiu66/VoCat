package device

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RecoverModem stops and restarts the cellular modem remoteproc so the SIM
// interface is re-initialized. A hot SIM swap on some modems leaves the UIM
// state stuck (no ATR / "possibly-removed") until the modem firmware is
// restarted; this performs that restart without rebooting the host. The modem
// is identified by its firmware image (mba.mbn), which distinguishes it from
// the WCNSS WiFi core. It is safe to call when no modem is present (no-op
// error) or while the modem is already running (it is restarted anyway).
func (manager *Manager) RecoverModem(ctx context.Context) error {
	// Serialize recoveries with each other and with in-flight UICC/APDU
	// transactions: restarting the modem while an eSIM operation is open would
	// leave both the kernel and the modem in an inconsistent state.
	manager.recoverMu.Lock()
	defer manager.recoverMu.Unlock()
	manager.uiccMu.Lock()
	defer manager.uiccMu.Unlock()

	const sysfsRoot = "/sys/class/remoteproc"
	entries, err := os.ReadDir(sysfsRoot)
	if err != nil {
		return fmt.Errorf("list remoteproc devices: %w", err)
	}
	var statePath, firmwareName string
	for _, entry := range entries {
		name := entry.Name()
		firmwarePath := filepath.Join(sysfsRoot, name, "firmware")
		data, readErr := os.ReadFile(firmwarePath)
		if readErr != nil {
			continue
		}
		firmware := strings.TrimSpace(string(data))
		if strings.Contains(strings.ToLower(firmware), "mba") {
			statePath = filepath.Join(sysfsRoot, name, "state")
			firmwareName = firmware
			break
		}
	}
	if statePath == "" {
		return errors.New("modem remoteproc not found")
	}
	if manager.logger != nil {
		manager.logger.Info("recovering modem: restarting modem remoteproc", "remoteproc", filepath.Base(filepath.Dir(statePath)), "firmware", firmwareName)
	}
	if err := os.WriteFile(statePath, []byte("stop"), 0); err != nil {
		return fmt.Errorf("stop modem: %w", err)
	}
	if err := waitForRemoteprocState(ctx, statePath, "offline"); err != nil {
		return fmt.Errorf("wait modem offline: %w", err)
	}
	if err := os.WriteFile(statePath, []byte("start"), 0); err != nil {
		return fmt.Errorf("start modem: %w", err)
	}
	return waitForRemoteprocState(ctx, statePath, "running")
}

func waitForRemoteprocState(ctx context.Context, statePath, want string) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			data, err := os.ReadFile(statePath)
			if err != nil {
				continue
			}
			if strings.TrimSpace(string(data)) == want {
				return nil
			}
		}
	}
}
