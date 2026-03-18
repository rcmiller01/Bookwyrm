package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"bookwyrm-launcher/internal/launcher"
	"github.com/kardianos/service"
)

type serviceProgram struct {
	cfg    launcher.Config
	cancel context.CancelFunc
	done   chan error
}

func (p *serviceProgram) Start(_ service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan error, 1)
	go func() {
		manager, err := launcher.NewManager(p.cfg)
		if err != nil {
			p.done <- err
			return
		}
		defer manager.Close()
		p.done <- manager.Run(ctx)
	}()
	return nil
}

func (p *serviceProgram) Stop(_ service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.done != nil {
		<-p.done
	}
	return nil
}

func main() {
	baseDir := ""
	filtered := []string{os.Args[0]}
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--base-dir" {
			if i+1 >= len(os.Args) {
				log.Fatal("--base-dir requires a value")
			}
			baseDir = os.Args[i+1]
			i++
			continue
		}
		filtered = append(filtered, os.Args[i])
	}
	os.Args = filtered
	cfg, err := launcher.LoadConfig(baseDir)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	command := "run"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "run":
		runStandalone(cfg)
	case "service-run":
		runAsService(cfg)
	case "install-service":
		controlService(cfg, "install")
	case "uninstall-service":
		controlService(cfg, "uninstall")
	case "start-service":
		controlService(cfg, "start")
	case "stop-service":
		controlService(cfg, "stop")
	case "status":
		controlService(cfg, "status")
	default:
		log.Fatalf("unknown command: %s", command)
	}
}

func runStandalone(cfg launcher.Config) {
	manager, err := launcher.NewManager(cfg)
	if err != nil {
		log.Fatalf("launcher init: %v", err)
	}
	defer manager.Close()
	if err := manager.Run(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("launcher failed: %v", err)
	}
}

func serviceConfig(cfg launcher.Config) *service.Config {
	return &service.Config{
		Name:        cfg.ServiceName,
		DisplayName: "Bookwyrm",
		Description: "Bookwyrm launcher service supervising metadata/indexer/backend",
		Arguments:   []string{"service-run", "--base-dir", filepath.Clean(cfg.BaseDir)},
	}
}

func runAsService(cfg launcher.Config) {
	program := &serviceProgram{cfg: cfg}
	svc, err := service.New(program, serviceConfig(cfg))
	if err != nil {
		log.Fatalf("create service: %v", err)
	}
	if err := svc.Run(); err != nil {
		log.Fatalf("service run failed: %v", err)
	}
}

func controlService(cfg launcher.Config, action string) {
	program := &serviceProgram{cfg: cfg}
	svc, err := service.New(program, serviceConfig(cfg))
	if err != nil {
		log.Fatalf("create service: %v", err)
	}
	if action == "status" {
		state, statusErr := svc.Status()
		if statusErr != nil {
			log.Fatalf("service status failed: %v", statusErr)
		}
		fmt.Printf("service status: %v\n", state)
		return
	}
	if err := service.Control(svc, action); err != nil {
		log.Fatalf("%s failed: %v", action, err)
	}
	fmt.Printf("%s succeeded\n", action)
}
