package telemetry

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"runtime"
	"time"
)

func TrackInstallation(version string) {
	go func() {
		hostname, _ := os.Hostname()
		hash := sha256.Sum256([]byte(hostname))
		instanceHash := hex.EncodeToString(hash[:8])
		req, _ := http.NewRequest("POST", "https://license.rms-ds.com/api/v1/telemetry/ping", nil)
		req.Header.Set("X-RMS-Telemetry", instanceHash+"|"+version+"|"+runtime.GOOS)
		resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
		if err != nil {
			return
		}
		resp.Body.Close()
	}()
}
