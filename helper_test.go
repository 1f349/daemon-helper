package daemonHelper

import (
	"github.com/1f349/tlogger"
	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"testing"
)

type testDaemon struct {
	Started           bool
	Stopped           bool
	ReloadStartCount  int
	ReloadFinishCount int
	sChan             chan struct{}
}

func (t *testDaemon) BuildUp(startup bool, logger *log.Logger) {
	if startup {
		t.Started = true
		t.sChan <- struct{}{}
	} else {
		t.ReloadStartCount++
		t.sChan <- struct{}{}
	}
}

func (t *testDaemon) TearDown(stopping bool, logger *log.Logger) {
	if stopping {
		t.Stopped = true
		t.sChan <- struct{}{}
	} else {
		t.ReloadFinishCount++
	}
}

func checkDaemonHelperTest(t *testing.T, d *testDaemon, expectedStarted, expectedStopped bool, expectedReloadCount int) {
	assert.Equal(t, expectedStarted, d.Started)
	assert.Equal(t, expectedStopped, d.Stopped)
	assert.Equal(t, expectedReloadCount, d.ReloadStartCount)
	assert.Equal(t, expectedReloadCount, d.ReloadFinishCount)
}

func TestNewDaemonRunner(t *testing.T) {
	d := &testDaemon{sChan: make(chan struct{})}
	dRunner := NewDaemonRunner(d, tlogger.NewTLoggerWithOptions(t, log.Options{
		Level: log.DebugLevel,
	}))
	assert.NotNil(t, dRunner)

	checkDaemonHelperTest(t, d, false, false, 0)

	go dRunner.Begin()
	<-d.sChan
	checkDaemonHelperTest(t, d, true, false, 0)

	assert.True(t, dRunner.Active())

	dRunner.SignalReload()
	<-d.sChan
	checkDaemonHelperTest(t, d, true, false, 1)

	dRunner.SignalReload()
	<-d.sChan
	checkDaemonHelperTest(t, d, true, false, 2)

	dRunner.SignalShutdown()
	<-d.sChan
	checkDaemonHelperTest(t, d, true, true, 2)

	dRunner.SignalReload()
	checkDaemonHelperTest(t, d, true, true, 2)
}
