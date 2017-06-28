package collator

import (
	"context"
	"github.com/gilramir/monitor-weblog/xojoc/logparse"
)

// Parse one line from a log file and send the Entry object for it.
func (self *Collator) _parse(ctx context.Context, lineChan <-chan string, entryChan chan<- *logparse.Entry) {
	defer close(entryChan)

	for {
		select {
		case <-ctx.Done():
			return

		case line, ok := <-lineChan:
			// Is the channel closed?
			if !ok {
				return
			}
			entry, err := logparse.Common(line)
			if err != nil {
				// We encountered an error; notify the listener and abort
				self.ErrorChan <- err
				return
			}
			entryChan <- entry
		}
	}
}
