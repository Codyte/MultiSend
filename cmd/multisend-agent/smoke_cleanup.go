package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func cleanupLabSmokeArtifacts(apiBase, jobID, manifestPath, runDir, runsRoot string) error {
	var errs []error
	if err := cleanupSmokeOperation(apiBase, "jobs", jobID); err != nil {
		errs = append(errs, fmt.Errorf("job history: %w", err))
	}
	if err := removeSmokeManifest(manifestPath); err != nil {
		errs = append(errs, fmt.Errorf("sender manifest: %w", err))
	}
	if err := removeSmokeRunTree(runDir, runsRoot); err != nil {
		errs = append(errs, fmt.Errorf("run directory: %w", err))
	}
	return errors.Join(errs...)
}

func cleanupDownloadSmokeArtifacts(apiBase, id, runDir, runsRoot string) error {
	var errs []error
	if err := cleanupSmokeOperation(apiBase, "downloads", id); err != nil {
		errs = append(errs, fmt.Errorf("download history: %w", err))
	}
	if err := removeSmokeRunTree(runDir, runsRoot); err != nil {
		errs = append(errs, fmt.Errorf("run directory: %w", err))
	}
	return errors.Join(errs...)
}

func cleanupSmokeOperation(apiBase, collection, id string) error {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	base := fmt.Sprintf("%s/%s/%s", strings.TrimRight(apiBase, "/"), collection, id)
	var state struct {
		Status string `json:"status"`
	}
	if err := apiGET(base, &state); err != nil {
		return err
	}
	if !terminalSmokeStatus(state.Status) {
		_ = apiPOST(base+"/cancel", map[string]any{}, nil)
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(200 * time.Millisecond)
			if err := apiGET(base, &state); err == nil && terminalSmokeStatus(state.Status) {
				break
			}
		}
	}
	if !terminalSmokeStatus(state.Status) {
		return fmt.Errorf("operation %s did not become terminal", id)
	}
	return apiDELETE(base)
}

func terminalSmokeStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "done", "completed", "failed", "canceled":
		return true
	default:
		return false
	}
}

func apiDELETE(url string) error {
	req, err := http.NewRequest(http.MethodDelete, url, nil) // #nosec G107 local fixed URL
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("DELETE %s returned status %d", url, resp.StatusCode)
	}
	return nil
}

func removeSmokeRunTree(runDir, runsRoot string) error {
	runAbs, err := filepath.Abs(runDir)
	if err != nil {
		return err
	}
	rootAbs, err := filepath.Abs(runsRoot)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, runAbs)
	if err != nil || rel == "." || filepath.Dir(rel) != "." || !strings.HasPrefix(filepath.Base(rel), "run-") {
		return fmt.Errorf("unsafe smoke run path: %s", runDir)
	}
	return os.RemoveAll(runAbs)
}

func removeSmokeManifest(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	profile := os.Getenv("USERPROFILE")
	if profile == "" {
		profile = `C:\Users\Public`
	}
	expectedRoot := filepath.Join(profile, "Downloads", "MultiSend", "manifests")
	abs, err := filepath.Abs(path)
	if err != nil || !samePath(filepath.Dir(abs), expectedRoot) || !strings.HasSuffix(strings.ToLower(filepath.Base(abs)), ".manifest.json") {
		return fmt.Errorf("unsafe smoke manifest path: %s", path)
	}
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
