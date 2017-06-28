package collator

import (
	"context"
	. "gopkg.in/check.v1"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"
)

func (s *MySuite) TestAlert(c *C) {
	// Create a junk file so that it exists for the Collator
	tmpFile := filepath.Join(s.tmpDir, "TestAlert")
	err := ioutil.WriteFile(tmpFile, []byte{'t', 'e', 's', 't', '\n'}, 0666)
	c.Assert(err, IsNil)

	// Start a collator
	ctx, cancelFunc := context.WithCancel(context.Background())
	m, err := NewAndRun(ctx, tmpFile, 10)
	c.Assert(err, IsNil)
	defer cancelFunc()

	// Add some big hit counts; we could be Add-ing while the
	// _collate go-routine is also Add-ing, but it's safe enough not
	// to worry about, and anyway, we're faster than the 1-second timer
	m.hitsMovingAverage.Add(20.0)
	m.hitsMovingAverage.Add(20.0)
	m.hitsMovingAverage.Add(20.0)

	// Expect an alert
	alert, err := getAlertWithTimeout(m, time.Duration(30)*time.Second)
	c.Assert(err, IsNil)
	c.Assert(alert, NotNil)
	c.Check(alert.InAlertState, Equals, true)
	log.Printf("Alerted moving average is %f", alert.AverageHitsPerSecond)
	c.Check(alert.AverageHitsPerSecond > 10.0, Equals, true)

	// Expect the recovery from an alert
	// Add one zero, first, to reduce the moving average
	m.hitsMovingAverage.Add(0.0)
	alert, err = getAlertWithTimeout(m, time.Duration(30)*time.Second)

	// Stop the Collator
	cancelFunc()

	c.Assert(err, IsNil)
	c.Assert(alert, NotNil)
	c.Check(alert.InAlertState, Equals, false)
	log.Printf("Recovered moving average is %f", alert.AverageHitsPerSecond)
	c.Check(alert.AverageHitsPerSecond <= 10.0, Equals, true)
}

func (s *MySuite) TestNoAlert(c *C) {
	// Create a junk file so that it exists for the Collator
	tmpFile := filepath.Join(s.tmpDir, "TestAlert")
	err := ioutil.WriteFile(tmpFile, []byte{'t', 'e', 's', 't', '\n'}, 0666)
	c.Assert(err, IsNil)

	// Start a collator
	ctx, cancelFunc := context.WithCancel(context.Background())
	m, err := NewAndRun(ctx, tmpFile, 10)
	c.Assert(err, IsNil)
	defer cancelFunc()

	// Add some big hit counts; we could be Add-ing while the
	// _collate go-routine is also Add-ing, but it's safe enough not
	// to worry about, and anyway, we're faster than the 1-second timer
	m.hitsMovingAverage.Add(8.0)
	m.hitsMovingAverage.Add(8.0)
	m.hitsMovingAverage.Add(8.0)

	// Expect no alert to be issued; we will have timed out instead.
	alert, err := getAlertWithTimeout(m, time.Duration(2)*time.Second)

	// Stop the Collator
	cancelFunc()
	c.Assert(err, IsNil)
	c.Assert(alert, IsNil)
}

// Wait for an Alert, but also time out after timeoutDuration, and return nil
// If an error was received from the Collator, return it too.
func getAlertWithTimeout(m *Collator, timeoutDuration time.Duration) (*Alert, error) {
	// Use a timeout so that in case the Collator blocks
	// forever, we can abort the test
	timeout := time.NewTimer(timeoutDuration)

	var alert *Alert

	for {
		select {
		case <-timeout.C:
			return nil, nil
		case err := <-m.ErrorChan:
			log.Printf("Received error from ErrorChan: %s", err)
			return nil, err
		case <-m.SitesChan:
			// ignore it
		case <-m.StatusChan:
			// ignore it
		case alert = <-m.AlertChan:
			timeout.Stop()
			return alert, nil
		}
	}
}
