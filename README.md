The program
===========
The monitor-weblog program was created more as an exercise in learning the
excellent termui package, than creating an extensible, configurable, useful
tool. However, it still does show the change in web traffice in real time,
so it still might be useful. At least it's useful as an example of how to
use termui.

This monitor-webglog program takes two arguements:

$ monitor-webglog pathToLogFile hitAlertLevel

pathToLogFile - the path to the log file to monitor
hitAlertLevel - the number of hits per second, over a 2-minute average, at which to issue alerts

In addition to alerts and the site counters, it also shows the latest hits per second, every second,
and the 2-minute moving average hits per second, every second, as line charts.

3rd party code
==============
Third party code is in the vendor directory, except for xojoc.pw/logparse
(which is also visible on github.com/xojoc/logparse). There's currently
an issue in "go get"ting it from xojoc.pw, so I have placed a copy
in xojoc/logparse.

Unit Tests
==========
So far there are only unit tests for the Alert logic. To test:

$ cd ${GOPATH}/src/github.com/gilramir/monitor-weblog/collator
$ go test

The tests use the "check" module, which I like to use for unit tests. You can also
see the test names as they run with;

$ go test -check.v

Design
======
(see dataflow.png)

The Collator monitors the log file and sends information via channels to the consumer.
If an error occurs during processing, it sends that error to the consumer so that it
can be reported to the user.  It can also receive a request from a channel to reset
its Sites counters. Its operation can be stopped by canceling the Context under
which it operates.

The UI logic receives info from the Collator, and updates the screen. It also listens for
input from the user via keyboard to either reset the sites counters, or to quit.

Possible future enhancements:

The hard-coded numbers could be configured via a configuration file

The Tail and Pares go-routines could be merged into one, as there's no need to
separate them. However, I kept them separate just to organize the logic.

The Status struct could be amended to send any particular interesting info.

The panes in the UI could be resized according to the wishes of the user.

Bugs
====
One issue is that the List widget does not seems to scroll, so if there are too many Alerts, not all
will be viewable. Much extra logic will be needed to implement scrolling. I would have to
keep track of which is the first item to be viewed, how many there are to view, and watch for
keyboard input for scrolling up and down.

