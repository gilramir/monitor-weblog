package collator

import (
	"context"
	"github.com/hpcloud/tail"
)

// see https://stackoverflow.com/questions/10135738/reading-log-files-as-theyre-updated-in-go

func (self *Collator) startTail(ctx context.Context, filename string, lineChan chan<- string) error {
	var err error
	self.tailer, err = tail.TailFile(filename, tail.Config{
		Follow:   true,                  // monitor for new lines (tail -f)
		ReOpen:   true,                  // re-open recreated files (taile -F)
		Location: &tail.SeekInfo{0, 2},  // start at the very end of the file
		Logger:   tail.DiscardingLogger, // we don't want logging to go to the console
	})
	if err != nil {
		return err
	}

	go self._tail(ctx, lineChan)
	return nil
}

// Watch (tail) the file and send one line of text when it is available
func (self *Collator) _tail(ctx context.Context, lineChan chan<- string) {
	defer close(lineChan)

	// "tail" the file
	for {
		select {
		case <-ctx.Done():
			return
		case tailLine, ok := <-self.tailer.Lines:
			if !ok {
				err := self.tailer.Err()
				if err != nil {
					// We encountered an error; notify the listener and abort
					self.ErrorChan <- err
					return
				}
				return
			}
			lineChan <- tailLine.Text
		}
	}
}
