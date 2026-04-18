// main.go — entry point for the FlexRadio Bubble Tea client.

package main

import (
	"flag"

	"goFlex/app"
)

func main() {
	var addr string
	var maxLog int
	flag.StringVar(&addr, "addr", "192.168.50.151", "FlexRadio address (host or host:port)")
	flag.IntVar(&maxLog, "log-lines", 500, "maximum log entries to retain")
	flag.Parse()

	app.Run(addr, maxLog)
}
