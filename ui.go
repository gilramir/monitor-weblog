package main

import (
	"context"
	"fmt"
	"github.com/gilramir/monitor-weblog/collator"
	"github.com/gizak/termui"
	"github.com/pkg/errors"
	"strconv"
)

// A container for the widgets we need to keep track of
type widgetCollection struct {
	sites  *termui.List
	alerts *termui.List
	hits   *termui.LineChart
	avg    *termui.LineChart
}

// Run the UI and return when it is stopped
func runUI(cancelFunc context.CancelFunc, c *collator.Collator) error {

	err := termui.Init()
	if err != nil {
		return errors.Wrap(err, "Starting console UI")
	}
	defer termui.Close()

	// This will hold any error returned from the Collator
	var collatorError error

	// Set up the UI
	widgets := createUI()
	setupEvents(c, widgets, &collatorError)

	// Start custom event producers that listen for messages
	// from the Collator
	go _watchSitesChannel(c)
	go _watchAlertChannel(c)
	go _watchErrorChannel(c)
	go _watchStatusChannel(c)

	// Start the UI event loop; it blocks until StopLoop is called
	termui.Loop()

	// Stop the Collator
	cancelFunc()

	return collatorError
}

// These go-routines simply wait for communication from the Collator,
// and then send the approriate event to the UI event loop.
func _watchSitesChannel(c *collator.Collator) {
	for stats := range c.SitesChan {
		termui.SendCustomEvt("/custom/sites", stats)
	}
}
func _watchAlertChannel(c *collator.Collator) {
	for alert := range c.AlertChan {
		termui.SendCustomEvt("/custom/alert", alert)
	}
}
func _watchErrorChannel(c *collator.Collator) {
	for err := range c.ErrorChan {
		termui.SendCustomEvt("/custom/error", err)
	}
}
func _watchStatusChannel(c *collator.Collator) {
	for err := range c.StatusChan {
		termui.SendCustomEvt("/custom/status", err)
	}
}

// Set up the UI
func createUI() *widgetCollection {
	// The widget holding the list of most visited sites
	sitesWidget := termui.NewList()
	sitesWidget.BorderLabel = "Highest Visited Sites"
	alertsWidget := termui.NewList()
	alertsWidget.BorderLabel = "Recent Alerts"

	// The widget holding the line chart of recent hits per second
	hitsWidget := termui.NewLineChart()
	hitsWidget.Mode = "dot"
	hitsWidget.BorderLabel = "Hits Per Second"
	hitsWidget.LineColor = termui.ColorYellow | termui.AttrBold
	hitsWidget.DataLabels = make([]string, 0)

	// The widget holding the line chart of 2-minut moving average hits per second
	avgWidget := termui.NewLineChart()
	avgWidget.Mode = "dot"
	avgWidget.BorderLabel = "2-minute Moving Average of Hits Per Second"
	avgWidget.LineColor = termui.ColorGreen | termui.AttrBold
	avgWidget.DataLabels = make([]string, 0)

	// The widget holding the one line of user instructions
	instructionsWidget := termui.NewPar("PRESS <ESC> or q TO QUIT, r TO RESET VISITED SITES COUNTERS")
	instructionsWidget.TextFgColor = termui.ColorRed
	instructionsWidget.BorderFg = termui.ColorCyan
	instructionsWidget.Height = 3

	widgets := &widgetCollection{
		sites:  sitesWidget,
		alerts: alertsWidget,
		hits:   hitsWidget,
		avg:    avgWidget,
	}
	resizeWidgets(widgets, termui.TermHeight())

	// Build the UI with a grid layout
	termui.Body.AddRows(
		termui.NewRow(
			termui.NewCol(12, 0, hitsWidget),
		),
		termui.NewRow(
			termui.NewCol(12, 0, avgWidget),
		),
		termui.NewRow(
			termui.NewCol(6, 0, sitesWidget),
			termui.NewCol(6, 0, alertsWidget),
		),
		termui.NewRow(
			termui.NewCol(12, 0, instructionsWidget),
		),
	)

	// Calculate the layout
	termui.Body.Align()

	// Render
	termui.Render(termui.Body)

	return widgets
}

// Size or re-size the widgets. This is used both when the UI is constructed
// for the first time, and when the window is re-sized.
func resizeWidgets(widgets *widgetCollection, screenHeight int) {
	// The instructions use 3 lines at the bottom of the screen
	usableHeight := float64(screenHeight - 3)

	widgets.sites.Height = int(usableHeight * 0.5)
	widgets.alerts.Height = int(usableHeight * 0.5)
	widgets.hits.Height = int(usableHeight * 0.25)
	widgets.avg.Height = int(usableHeight * 0.25)
}

// Connect the UI events to actions to be taken when those events come in.
func setupEvents(c *collator.Collator, widgets *widgetCollection, collatorError *error) {

	// <ESC> to quit
	termui.Handle("/sys/kbd/<escape>", func(termui.Event) {
		termui.StopLoop()
	})

	// <q> to quit
	termui.Handle("/sys/kbd/q", func(termui.Event) {
		termui.StopLoop()
	})

	// r to reset
	termui.Handle("/sys/kbd/r", func(termui.Event) {
		c.ResetChan <- true
		widgets.sites.Items = []string{}
		termui.Render(widgets.sites)
	})

	// Sites data
	termui.Handle("/custom/sites", func(e termui.Event) {
		updateSitesWidget(widgets.sites, e.Data.(*collator.Sites))
	})

	// Alert data
	termui.Handle("/custom/alert", func(e termui.Event) {
		updateAlertsWidget(widgets.alerts, e.Data.(*collator.Alert))
	})

	// Status data
	termui.Handle("/custom/status", func(e termui.Event) {
		updateHitsWidget(widgets.hits, e.Data.(*collator.Status))
		updateAvgWidget(widgets.avg, e.Data.(*collator.Status))
	})

	// Error from Collator
	termui.Handle("/custom/error", func(e termui.Event) {
		*collatorError = e.Data.(error)
		termui.StopLoop()
	})

	// Window geometry changed
	termui.Handle("/sys/wnd/resize", func(e termui.Event) {
		resizeWidgets(widgets, e.Data.(termui.EvtWnd).Height)
		termui.Body.Width = e.Data.(termui.EvtWnd).Width

		// Calculate the layout
		termui.Body.Align()
		// Render
		termui.Render(termui.Body)
	})
}

// Update the list of sites and their number of hits
func updateSitesWidget(stats *termui.List, sites *collator.Sites) {
	//	stats.Text = ""
	stats.Items = make([]string, len(sites.Sites))

	// Find the larget number of digits used to represent a number,
	// so we can build a format string and have the numbers aligned nicely.
	largestWidth := 0
	for _, site := range sites.Sites {
		thisWidth := len(strconv.Itoa(site.TotalHits))
		if thisWidth > largestWidth {
			largestWidth = thisWidth
		}
	}
	formatString := fmt.Sprintf("%%%dd: %%s\n", largestWidth)

	// Fill in the list of sites
	for i, site := range sites.Sites {
		stats.Items[i] = fmt.Sprintf(formatString, site.TotalHits, site.Site)
	}

	termui.Render(stats)
}

const (
	// The Golang way of saying Year-Month-Day Hour:Minute:Second.FractionalSecond
	kTimeFormat = "2006-01-02 15:04:05.000"
)

// Update the list of alerts on the screen.
// XXX - does this scroll? it seems it does not, and thus extra logic would
// be required to autoscroll and scroll this widget.
func updateAlertsWidget(alertsWidget *termui.List, alert *collator.Alert) {
	var newText string
	if alert.InAlertState {
		newText = fmt.Sprintf("%s [ALERT](fg-white,bg-red) High traffic; hits = %.1f/s\n",
			alert.Time.Format(kTimeFormat), alert.AverageHitsPerSecond)
	} else {
		newText = fmt.Sprintf("%s       Recovered, hits = %.1f/s\n",
			alert.Time.Format(kTimeFormat), alert.AverageHitsPerSecond)
	}

	alertsWidget.Items = append(alertsWidget.Items, newText)

	termui.Render(alertsWidget)
}

// Update the latest hits per second line chart
func updateHitsWidget(hitsWidget *termui.LineChart, status *collator.Status) {
	hitsWidget.Data = append(hitsWidget.Data, float64(status.HitsLastSecond))
	// If there are too many, remove some from the front
	if len(hitsWidget.Data) > hitsWidget.Width {
		hitsWidget.Data = hitsWidget.Data[1:]
	} else {
		// A little tricky... we only need to add as many empty labels as needed to
		// fill the width of the screen.
		hitsWidget.DataLabels = append(hitsWidget.DataLabels, "")
	}
	termui.Render(hitsWidget)
}

// Update the moving average hits per second line chart
func updateAvgWidget(avgWidget *termui.LineChart, status *collator.Status) {
	avgWidget.Data = append(avgWidget.Data, float64(status.AverageHitsPerSecond))
	// If there are too many, remove some from the front
	if len(avgWidget.Data) > avgWidget.Width {
		avgWidget.Data = avgWidget.Data[1:]
	} else {
		// A little tricky... we only need to add as many empty labels as needed to
		// fill the width of the screen.
		avgWidget.DataLabels = append(avgWidget.DataLabels, "")
	}
	termui.Render(avgWidget)
}
