package collator

// The Collator watches a log file in the Common Log format and sends data
// to a listener.

import (
	"context"
	"github.com/RobinUS2/golang-moving-average"
	"github.com/gilramir/monitor-weblog/xojoc/logparse"
	"github.com/hpcloud/tail"
	"sort"
	"strings"
	"time"
)

const (
	// How often to report Sites information
	kSitesTimerDuration = 10 * time.Second

	// How often to check the moving average of hits, and thus,
	// how often to check to see if we need to send an alert. This is also
	// used to send Status objects.
	kMovingAverageTimerDuration = 1 * time.Second
)

// An Alert notifies the listener of high traffic, and also when traffic
// returns to a normal level
type Alert struct {
	InAlertState         bool
	AverageHitsPerSecond float64
	Time                 time.Time
}

// The Sites object lists the # of hits per site, and are sent less often
// (every 10 seconds)
type Sites struct {
	Sites []Site
}

type Site struct {
	TotalHits int
	Site      string
}

// These Status objects are sent frequently (one per second)
type Status struct {
	HitsLastSecond       int
	AverageHitsPerSecond float64
	// Additional information could be added here in the future
}

// ByHits implements sort.Interface for []SizeHite, based on the number of hits
type ByHits []Site

func (a ByHits) Len() int           { return len(a) }
func (a ByHits) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByHits) Less(i, j int) bool { return a[i].TotalHits < a[j].TotalHits }

type Collator struct {
	ErrorChan  chan error
	SitesChan  chan *Sites
	StatusChan chan *Status
	AlertChan  chan *Alert
	ResetChan  chan bool

	alertThreshold float64
	tailer         *tail.Tail

	accumHits          int
	inAlertedState     bool
	sitesTimer         *time.Timer
	movingAverageTimer *time.Timer
	hitsMovingAverage  *movingaverage.MovingAverage

	siteHits map[string]int
}

// Create a new Collator and start running its goroutines. The caller can
// stop the Collator by calling the CancelFunc in the passed-in context.
func NewAndRun(ctx context.Context, filename string, alertThreshold int) (*Collator, error) {
	c := &Collator{
		ErrorChan:         make(chan error, 1), // buffered so anyone can write an error at any time
		SitesChan:         make(chan *Sites),
		AlertChan:         make(chan *Alert),
		StatusChan:        make(chan *Status),
		ResetChan:         make(chan bool),
		siteHits:          make(map[string]int),
		hitsMovingAverage: movingaverage.New(2 * 60), // 2 minutes, with 1-second windows
		alertThreshold:    float64(alertThreshold),
	}

	// Tail the log
	lineChan := make(chan string)
	err := c.startTail(ctx, filename, lineChan)
	if err != nil {
		return nil, err
	}

	// Parse each line
	entryChan := make(chan *logparse.Entry)
	go c._parse(ctx, lineChan, entryChan)

	// And monitor the information
	go c._collate(ctx, entryChan)

	return c, nil
}

func (self *Collator) _collate(ctx context.Context, entryChan <-chan *logparse.Entry) {
	defer self.tailer.Stop() // this will ignore a possible error, but that's ok
	defer close(self.ErrorChan)
	defer close(self.SitesChan)
	defer close(self.AlertChan)
	defer close(self.StatusChan)

	self.sitesTimer = time.NewTimer(kSitesTimerDuration)
	self.movingAverageTimer = time.NewTimer(kMovingAverageTimerDuration)

	for {
		select {
		// We've been told to stop working
		case <-ctx.Done():
			return

		// A log entry
		case entry := <-entryChan:
			self.recordEntry(entry)
			self.accumHits++

		// Moving Average timer
		case now := <-self.movingAverageTimer.C:
			// Calculate the 2-minute moving average
			self.hitsMovingAverage.Add(float64(self.accumHits))
			avg := self.hitsMovingAverage.Avg()

			// Send the per-second status
			self.StatusChan <- &Status{
				HitsLastSecond:       self.accumHits,
				AverageHitsPerSecond: avg,
			}

			self.accumHits = 0

			// Need to alert?
			if self.inAlertedState {
				if avg < self.alertThreshold {
					self.AlertChan <- &Alert{false, avg, now}
					self.inAlertedState = false
				}
			} else {
				if avg > self.alertThreshold {
					self.AlertChan <- &Alert{true, avg, now}
					self.inAlertedState = true
				}
			}

			self.movingAverageTimer.Reset(kMovingAverageTimerDuration)

		// Sitest timer
		case <-self.sitesTimer.C:
			self.sendSites()
			self.sitesTimer.Reset(kSitesTimerDuration)

		// User requests a reset of counters
		case <-self.ResetChan:
			self.siteHits = make(map[string]int)

		}
	}
}

// Given a single log entry, record any useful info from it.
func (self *Collator) recordEntry(entry *logparse.Entry) {

	// Sanity check
	if len(entry.Request.URL.Path) < 3 || entry.Request.URL.Path[0] != '/' {
		return
	}

	secondSlashIndex := strings.Index(entry.Request.URL.Path[1:], "/")
	if secondSlashIndex == -1 {
		// Not present
		return
	}
	// Add 1 to the index because the Index() call was made on a substring starting at position 1
	site := entry.Request.URL.Path[:secondSlashIndex+1]

	_, has := self.siteHits[site]
	if has {
		self.siteHits[site]++
	} else {
		self.siteHits[site] = 1
	}
}

// Send a Hit struct to the client
func (self *Collator) sendSites() {
	// Create the slice of Site's
	sites := make([]Site, len(self.siteHits))
	i := 0
	for site, totalHits := range self.siteHits {
		sites[i].Site = site
		sites[i].TotalHits = totalHits
		i++
	}
	// Reverse sort them by number of hits per site
	sort.Sort(sort.Reverse(ByHits(sites)))
	self.SitesChan <- &Sites{
		Sites: sites,
	}
}
