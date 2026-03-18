package launcher

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	cfg      Config
	log      *log.Logger
	logClose func() error
}

type managedProcessState struct {
	cfg    ServiceConfig
	cmd    *exec.Cmd
	logF   *os.File
	policy RestartPolicy
}

func NewManager(cfg Config) (*Manager, error) {
	logger, closer, err := NewLauncherLogger(filepath.Join(cfg.LogDir, "launcher.log"))
	if err != nil {
		return nil, err
	}
	return &Manager{
		cfg: cfg,
		log: logger,
		logClose: func() error {
			if closer != nil {
				return closer.Close()
			}
			return nil
		},
	}, nil
}

func (m *Manager) Close() error {
	return m.logClose()
}

func (m *Manager) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	m.logPathDiagnostics()

	states := make([]*managedProcessState, 0, len(m.cfg.Services))
	for _, svc := range m.cfg.Services {
		states = append(states, &managedProcessState{
			cfg: svc,
			policy: RestartPolicy{
				Limit:     m.cfg.RestartLimit,
				Window:    m.cfg.RestartWindow,
				BaseDelay: m.cfg.RestartBaseDelay,
				MaxDelay:  m.cfg.RestartMaxDelay,
			},
		})
	}

	var mu sync.Mutex
	shuttingDown := false
	errCh := make(chan error, len(states))
	var wg sync.WaitGroup

	startProc := func(st *managedProcessState) error {
		if strings.TrimSpace(st.cfg.Executable) == "" {
			return fmt.Errorf("%s executable not configured", st.cfg.Name)
		}
		logFile, err := openRotatingFile(st.cfg.LogFile, defaultRotateSize, defaultRotateKeep)
		if err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, st.cfg.Executable, st.cfg.Args...)
		cmd.Dir = filepath.Dir(st.cfg.Executable)
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		cmd.Env = m.childEnv()
		if err := cmd.Start(); err != nil {
			_ = logFile.Close()
			return err
		}
		st.cmd = cmd
		st.logF = logFile
		m.log.Printf("started %s (pid=%d)", st.cfg.Name, cmd.Process.Pid)
		return nil
	}

	for _, state := range states {
		wg.Add(1)
		go func(st *managedProcessState) {
			defer wg.Done()
			for {
				if err := startProc(st); err != nil {
					errCh <- fmt.Errorf("%s start failed: %w", st.cfg.Name, err)
					return
				}
				waitErr := st.cmd.Wait()
				if st.logF != nil {
					_ = st.logF.Close()
					st.logF = nil
				}
				mu.Lock()
				stopNow := shuttingDown
				mu.Unlock()
				if stopNow || ctx.Err() != nil {
					return
				}
				m.log.Printf("%s exited: %v", st.cfg.Name, waitErr)
				allow, delay := st.policy.AllowRestart(time.Now().UTC())
				if !allow {
					errCh <- fmt.Errorf("%s exceeded restart limit in %s", st.cfg.Name, st.policy.Window)
					return
				}
				m.log.Printf("restarting %s in %s", st.cfg.Name, delay)
				select {
				case <-ctx.Done():
					return
				case <-time.After(delay):
				}
			}
		}(state)
	}

	healthCtx, healthCancel := context.WithTimeout(ctx, m.cfg.HealthTimeout)
	defer healthCancel()
	healthEndpoints := make([]string, 0, len(states))
	for _, st := range states {
		healthEndpoints = append(healthEndpoints, st.cfg.HealthURL)
	}
	if err := WaitForHealthy(healthCtx, &http.Client{Timeout: 3 * time.Second}, healthEndpoints, 1500*time.Millisecond); err != nil {
		cancel()
		wg.Wait()
		return err
	}
	m.log.Printf("all services healthy")
	_ = m.handleFirstRun()

	select {
	case <-ctx.Done():
	case runErr := <-errCh:
		cancel()
		m.stopAll(states, &mu, &shuttingDown)
		wg.Wait()
		return runErr
	}
	m.stopAll(states, &mu, &shuttingDown)
	wg.Wait()
	return nil
}

func (m *Manager) logPathDiagnostics() {
	if runtime.GOOS != "windows" {
		return
	}
	libraryRoot := strings.TrimSpace(m.cfg.Env["LIBRARY_ROOT"])
	if libraryRoot == "" {
		m.log.Printf("startup warning: LIBRARY_ROOT is not configured in launcher env")
		return
	}
	libraryRoot = filepath.Clean(libraryRoot)
	if !filepath.IsAbs(libraryRoot) {
		m.log.Printf("startup warning: LIBRARY_ROOT should be absolute on Windows: %s", libraryRoot)
	}
	if len(libraryRoot) > 240 {
		m.log.Printf("startup warning: LIBRARY_ROOT path length exceeds 240 chars: %s", libraryRoot)
	}
	if info, err := os.Stat(libraryRoot); err != nil || !info.IsDir() {
		m.log.Printf("startup warning: LIBRARY_ROOT path missing or invalid: %s", libraryRoot)
		return
	}
	testFile := filepath.Join(libraryRoot, ".bookwyrm-launcher-write-test")
	if err := os.WriteFile(testFile, []byte("ok"), 0o644); err != nil {
		m.log.Printf("startup warning: LIBRARY_ROOT not writable: %s", libraryRoot)
	} else {
		_ = os.Remove(testFile)
	}
	downloadsPath := strings.TrimSpace(m.cfg.Env["DOWNLOADS_COMPLETED_PATH"])
	if downloadsPath != "" {
		if !filepath.IsAbs(downloadsPath) {
			m.log.Printf("startup warning: DOWNLOADS_COMPLETED_PATH should be absolute: %s", downloadsPath)
		}
		if info, err := os.Stat(downloadsPath); err != nil || !info.IsDir() {
			m.log.Printf("startup warning: DOWNLOADS_COMPLETED_PATH missing or invalid: %s", downloadsPath)
		}
	}
}

func (m *Manager) stopAll(states []*managedProcessState, mu *sync.Mutex, shuttingDown *bool) {
	mu.Lock()
	*shuttingDown = true
	mu.Unlock()

	deadline := time.Now().Add(m.cfg.StopTimeout)
	for _, st := range states {
		if st.cmd == nil || st.cmd.Process == nil {
			continue
		}
		_ = st.cmd.Process.Signal(os.Interrupt)
	}
	for _, st := range states {
		if st.cmd == nil || st.cmd.Process == nil {
			continue
		}
		for time.Now().Before(deadline) {
			if st.cmd.ProcessState != nil && st.cmd.ProcessState.Exited() {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		_ = st.cmd.Process.Kill()
	}
}

func (m *Manager) childEnv() []string {
	merged := map[string]string{}
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			merged[parts[0]] = parts[1]
		}
	}
	for k, v := range m.cfg.Env {
		merged[k] = v
	}
	if _, ok := merged["BOOKWYRM_LOG_DIR"]; !ok {
		merged["BOOKWYRM_LOG_DIR"] = m.cfg.LogDir
	}
	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	return out
}

func (m *Manager) handleFirstRun() error {
	statePath := filepath.Join(m.cfg.DataDir, "first_run_complete.json")
	state, err := loadFirstRunState(statePath)
	if err != nil {
		return err
	}
	if state.Complete || !m.cfg.OpenBrowserOnFirstStart {
		return nil
	}
	if err := OpenBrowser(m.cfg.LaunchURL); err != nil {
		m.log.Printf("failed to open browser: %v", err)
		return nil
	}
	state.Complete = true
	state.Completed = time.Now().UTC()
	return saveFirstRunState(statePath, state)
}
