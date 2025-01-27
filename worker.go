//go:build !windows
// +build !windows

package graceful

import (
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	workerStopSignal = syscall.SIGTERM
)

var (
	ErrNoServers = errors.New("no servers")
)

type worker struct {
	handlers []*fiber.App
	servers  []server
	opt      *option
	stopCh   chan struct{}
	sync.Mutex
}

type server struct {
	*fiber.App
	listener net.Listener
}

func (w *worker) run() error {
	// init servers with fds from master
	err := w.initServers()
	if err != nil {
		return err
	}

	// start http servers
	err = w.startServers()
	if err != nil {
		return err
	}

	oldWorkerPid, err := strconv.Atoi(os.Getenv(EnvOldWorkerPid))
	if err == nil && oldWorkerPid > 1 {
		// tell old worker i'm ready, you should go away
		err = syscall.Kill(oldWorkerPid, workerStopSignal)
		if err != nil {
			// unexpected: kill old worker fail
			log.Printf("[warning] kill old worker error: %v\n", err)
		}
	}

	go w.watchMaster()

	// waitSignal
	w.waitSignal()
	return nil
}

func (w *worker) initServers() error {
	numFDs, err := strconv.Atoi(os.Getenv(EnvNumFD))
	if err != nil {
		return fmt.Errorf("invalid %s integer", EnvNumFD)
	}

	if len(w.handlers) != numFDs {
		return fmt.Errorf("handler number does not match numFDs, %v!=%v", len(w.handlers), numFDs)
	}

	for i := 0; i < numFDs; i++ {
		f := os.NewFile(uintptr(3+i), "") // fd start from 3
		l, err := net.FileListener(f)
		if err != nil {
			return fmt.Errorf("failed to inherit file descriptor: %d", i)
		}
		server := server{
			App:      w.handlers[i],
			listener: l,
		}
		w.servers = append(w.servers, server)
	}
	return nil
}

func (w *worker) startServers() error {
	if len(w.servers) == 0 {
		return ErrNoServers
	}
	for i := 0; i < len(w.servers); i++ {
		s := w.servers[i]
		go func() {
			if err := s.Listener(s.listener); err != nil {
				log.Printf("http Serve error: %v\n", err)
			}
		}()
	}

	return nil
}

// watchMaster to monitor if master dead
func (w *worker) watchMaster() error {
	masterPid := os.Getenv(EnvParentPid)
	for {
		// if parent id change to 1, it means parent is dead
		if !processExist(masterPid) {
			log.Printf("master dead, stop worker\n")
			w.stop()
			break
		}
		time.Sleep(w.opt.watchInterval)
	}
	w.stopCh <- struct{}{}
	return nil
}

func processExist(pid string) bool {
	Pid, _ := strconv.ParseInt(pid, 10, 64)
	if err := syscall.Kill(int(Pid), 0); err != nil {
		log.Printf("kill err: %v", err)
		return false
	}
	return true
}

func (w *worker) waitSignal() {
	ch := make(chan os.Signal)
	signal.Notify(ch, workerStopSignal)
	select {
	case sig := <-ch:
		log.Printf("worker got signal: %v\n", sig)
	case <-w.stopCh:
		log.Printf("stop worker")
	}

	w.stop()
}

// TODO: shutdown in parallel
func (w *worker) stop() {
	w.Lock()
	defer w.Unlock()
	for _, server := range w.servers {
		err := server.ShutdownWithTimeout(w.opt.stopTimeout)
		if err != nil {
			log.Printf("shutdown server error: %v\n", err)
		}
	}
}
