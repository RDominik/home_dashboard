package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

type stepResult struct {
	Step   string `json:"step"`
	OK     bool   `json:"ok"`
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
}

type updateResult struct {
	OK      bool         `json:"ok"`
	Results []stepResult `json:"results"`
}

func main() {
	var addr string
	var repoDir string
	var composeFile string

	flag.StringVar(&addr, "addr", ":8090", "listen address")
	flag.StringVar(&repoDir, "repo-dir", ".", "repository root directory")
	flag.StringVar(&composeFile, "compose-file", "docker-compose.webui.yml", "docker compose file path")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/system/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "method not allowed"})
			return
		}

		result := runUpdate(repoDir, composeFile)
		status := http.StatusOK
		if !result.OK {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, result)
	})

	log.Printf("systemUpdate service listening on %s (repo=%s, compose=%s)", addr, repoDir, composeFile)
	if err := http.ListenAndServe(addr, cors(mux)); err != nil {
		log.Fatal(err)
	}
}

func runUpdate(repoDir string, composeFile string) updateResult {
	if repoDir == "" {
		repoDir = "."
	}
	if composeFile == "" {
		composeFile = "docker-compose.webui.yml"
	}

	results := make([]stepResult, 0, 2)

	gitOut, gitErr := runCommand(repoDir, "git", "pull")
	results = append(results, stepResult{Step: "git pull", OK: gitErr == nil, Stdout: gitOut, Stderr: errorText(gitErr)})
	if gitErr != nil {
		return updateResult{OK: false, Results: results}
	}

	composeOut, composeErr := runCommand(repoDir, "docker", "compose", "-f", composeFile, "up", "--build", "-d")
	results = append(results, stepResult{Step: "docker compose up --build -d", OK: composeErr == nil, Stdout: composeOut, Stderr: errorText(composeErr)})

	allOK := true
	for _, result := range results {
		if !result.OK {
			allOK = false
		}
	}

	return updateResult{OK: allOK, Results: results}
}

func runCommand(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if len(text) > 500 {
		text = text[len(text)-500:]
	}
	return text, err
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("write json failed: %v", err)
	}
}