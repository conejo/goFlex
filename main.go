// main.go — entry point for the FlexRadio Bubble Tea client.

package main

import (
	"flag"

	"goFlex/app"
)

func main() {
	var maxLog int
	flag.IntVar(&maxLog, "log-lines", 500, "maximum log entries to retain")
	flag.Parse()

	app.Run(maxLog)
}
