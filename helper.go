package daemonHelper

import (
	"github.com/charmbracelet/log"
	"github.com/mrmelon54/rescheduler"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Daemon provides an interface to start and stop a daemon
type Daemon interface {
	BuildUp(startup bool, logger *log.Logger)
	TearDown(stopping bool, logger *log.Logger)
}

// DaemonRunner provides an interface to run a daemon and signal actions
type DaemonRunner interface {
	Begin()
	SignalReload()
	SignalShutdown()
	Starting() bool
	Reloading() bool
	Stopping() bool
	Active() bool
}

type defaultDaemonRunner struct {
	beginMutex  *sync.Mutex
	daemon      Daemon
	schedReload *rescheduler.Rescheduler
	sigChan     chan os.Signal
	doneChan    chan struct{}
	logger      *log.Logger
	isStarting  bool
	isReloading bool
	isStopping  bool
	isActive    bool
}

func (d *defaultDaemonRunner) Starting() bool {
	return d.isStarting
}

func (d *defaultDaemonRunner) Reloading() bool {
	return d.isReloading
}

func (d *defaultDaemonRunner) Stopping() bool {
	return d.isStopping
}

func (d *defaultDaemonRunner) Active() bool {
	return d.isActive && !d.isReloading
}

// SignalReload tells the Daemon to reload
func (d *defaultDaemonRunner) SignalReload() {
	d.logger.Debug("Signalling Reload...")
	d.sigChan <- syscall.SIGHUP
	d.logger.Debug("Signalling Reload Complete!")
}

// SignalShutdown tells the Daemon to tearDown
func (d *defaultDaemonRunner) SignalShutdown() {
	d.logger.Debug("Signalling Termination...")
	d.sigChan <- syscall.SIGTERM
	d.logger.Debug("Signalling Termination Complete!")
}

// Begin starts the Daemon and blocks until it shuts-down
func (d *defaultDaemonRunner) Begin() {
	d.beginMutex.Lock()
	defer d.beginMutex.Unlock()
	d.isStarting = true
	d.logger.Debug("Hello")
	d.logger.Info("Starting up...")
	n := time.Now()
	d.logger.Debug("Setting up signals...")
	d.sigChan = make(chan os.Signal, 1)
	go func() {
		for {
			select {
			case sig := <-d.sigChan:
				switch sig {
				case syscall.SIGHUP:
					d.logger.Debug("Scheduling Reload!")
					d.schedReload.Run()
				default:
					d.logger.Debug("Waiting for reload completion...")
					d.schedReload.Wait()
					d.logger.Debug("Closing done channel...")
					close(d.doneChan)
					return
				}
			case <-d.doneChan:
				d.logger.Debug("Done channel closed!")
				return
			}
		}
	}()
	d.logger.Debug("Signal Setup Complete!")
	d.daemon.BuildUp(true, d.logger)
	d.isStarting = false
	d.logger.Infof("Took '%s' to startup", time.Now().Sub(n))
	d.isActive = true

	signal.Notify(d.sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, os.Interrupt, os.Kill)
	<-d.doneChan
	d.isActive = false
	d.isStopping = true
	d.logger.Info("Shutting down...")
	n = time.Now()
	d.daemon.TearDown(true, d.logger)
	d.logger.Infof("Took '%s' to shutdown", time.Now().Sub(n))
	log.Debug("Goodbye")
	d.isStopping = false
}

// NewDaemonRunner creates a new DaemonRunner implementation with the specified Daemon and logger.
func NewDaemonRunner(daemonIn Daemon, logger *log.Logger) DaemonRunner {
	if daemonIn == nil || logger == nil {
		return nil
	}
	ddRunner := &defaultDaemonRunner{
		daemon:     daemonIn,
		beginMutex: &sync.Mutex{},
		doneChan:   make(chan struct{}),
		logger:     logger,
	}
	ddRunner.schedReload = rescheduler.NewRescheduler(func() {
		ddRunner.isReloading = true
		logger.Debug("Reload Tear Down Start...")
		daemonIn.TearDown(false, logger)
		logger.Debug("Reload Tear Down Complete!")
		logger.Debug("Reload Build Up Start...")
		daemonIn.BuildUp(false, logger)
		logger.Debug("Reload Build Up Complete!")
		ddRunner.isReloading = false
	})
	return ddRunner
}
