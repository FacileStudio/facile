package authflow

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/FacileStudio/facile/internal/manifest"
	"github.com/FacileStudio/facile/internal/ui"
)

// defaultPollInterval is what a device flow uses when the server names none.
const defaultPollInterval = 5 * time.Second

// deviceLogin is the headless path: the machine being authorized never opens
// the identity provider itself.
func deviceLogin(a *manifest.Auth, serverURL string, opts Options) (string, error) {
	flow := a.Device
	verification, err := startDeviceFlow(serverURL, flow)
	if err != nil {
		return "", err
	}

	ui.Step("Confirm the code %s to authorize this machine", verification.userCode)
	showVerification(verification.verifyURL, opts)

	deadline := time.Now().Add(verification.expires)
	for time.Now().Before(deadline) {
		time.Sleep(verification.interval)
		token, done, err := pollDevice(serverURL, flow.PollPath, verification.deviceCode)
		if !done {
			continue
		}
		if err != nil {
			return "", err
		}
		return token, nil
	}
	return "", fmt.Errorf("the authorization timed out — run `facile login` again")
}

type deviceVerification struct {
	deviceCode string
	userCode   string
	verifyURL  string
	interval   time.Duration
	expires    time.Duration
}

// startDeviceFlow asks the provider to begin an authorization and reads how to
// poll for its result.
func startDeviceFlow(serverURL string, flow *manifest.DeviceFlow) (deviceVerification, error) {
	machine, _ := os.Hostname()

	res, err := post(serverURL+flow.StartPath, map[string]string{"machine": machine})
	if err != nil {
		return deviceVerification{}, err
	}
	if !res.ok() {
		return deviceVerification{}, fmt.Errorf("the server would not start an authorization (%d) — check the server URL", res.status)
	}

	doc := res.decode()
	deviceCode := stringField(doc, "device_code")
	if deviceCode == "" {
		return deviceVerification{}, fmt.Errorf("the server's device authorization was not usable — report it against the server")
	}
	return deviceVerification{
		deviceCode: deviceCode,
		userCode:   stringField(doc, "user_code"),
		verifyURL:  stringField(doc, "verification_uri_complete"),
		interval:   seconds(numberField(doc, "interval"), defaultPollInterval),
		expires:    seconds(numberField(doc, "expires_in"), 10*time.Minute),
	}, nil
}

// showVerification prints the URL to approve on, or opens the browser and asks
// the provider to poll in the background.
func showVerification(verifyURL string, opts Options) {
	if verifyURL != "" {
		if opts.NoBrowser || !openBrowser(verifyURL) {
			ui.Hint("open %s", verifyURL)
		} else {
			ui.Hint("if nothing opened: %s", verifyURL)
		}
	}
	ui.Step("Waiting for approval")
}

// pollDevice asks once whether the authorization completed. done is true when
// the provider gave a final answer — the token, or a denial — and false while
// it is still pending or the network hiccuped, so the caller keeps polling.
func pollDevice(serverURL, pollPath, deviceCode string) (string, bool, error) {
	res, err := post(serverURL+pollPath, map[string]string{"device_code": deviceCode})
	if err == nil {
		return deviceAnswer(res)
	}
	return "", false, nil
}

// deviceAnswer reads one poll response into the (token, done, err) shape.
func deviceAnswer(res response) (string, bool, error) {
	switch {
	case res.ok():
		token := stringField(res.decode(), "token")
		if token == "" {
			return "", true, fmt.Errorf("the server approved the machine but returned no token — report it against the server")
		}
		return token, true, nil
	case res.status == http.StatusBadRequest || res.status == http.StatusForbidden:
		return "", true, fmt.Errorf("the authorization was denied or expired — run `facile login` again")
	}
	return "", false, nil
}

func numberField(doc map[string]any, key string) int {
	value, _ := doc[key].(float64)
	return int(value)
}

// seconds reads a duration a server states in whole seconds, falling back when
// it states nothing usable. A zero interval would busy-poll a provider and a
// zero deadline would end the wait before it started.
func seconds(value int, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return time.Duration(value) * time.Second
}
