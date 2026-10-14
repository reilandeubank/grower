// Package push sends web push notifications: the daily care reminder and the test
// notification from Settings.
package push

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SherClockHolmes/webpush-go"
)

// KeysFile holds the VAPID key pair inside the data dir. Browsers tie each subscription
// to the public key, so losing the file means every device must turn reminders on again.
const KeysFile = "vapid.json"

// Keys is the server's VAPID key pair in URL-safe base64.
type Keys struct {
	Public  string `json:"public_key"`
	Private string `json:"private_key"`
}

// LoadKeys reads <dataDir>/vapid.json, generating it on first start.
func LoadKeys(dataDir string) (Keys, error) {
	path := filepath.Join(dataDir, KeysFile)
	b, err := os.ReadFile(path) //nolint:gosec // G304: our own data dir
	if errors.Is(err, os.ErrNotExist) {
		return createKeys(path)
	}
	if err != nil {
		return Keys{}, fmt.Errorf("read VAPID keys: %w", err)
	}
	var k Keys
	if err := json.Unmarshal(b, &k); err != nil || k.Public == "" || k.Private == "" {
		return Keys{}, fmt.Errorf("%s is damaged; restore it from a backup, or delete it to make new keys (devices must then turn reminders on again)", path)
	}
	return k, nil
}

// createKeys writes a new pair through a temp file and rename, so a crash never leaves
// a half-written key file behind.
func createKeys(path string) (Keys, error) {
	var k Keys
	var err error
	if k.Private, k.Public, err = webpush.GenerateVAPIDKeys(); err != nil {
		return Keys{}, fmt.Errorf("generate VAPID keys: %w", err)
	}
	b, err := json.MarshalIndent(k, "", "  ") //nolint:gosec // G117: writing the key file is the point; it is 0600
	if err != nil {
		return Keys{}, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return Keys{}, fmt.Errorf("write VAPID keys: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return Keys{}, fmt.Errorf("write VAPID keys: %w", err)
	}
	return k, nil
}
