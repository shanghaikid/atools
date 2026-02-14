// worldtime is a CLI tool that displays the current time for cities around the world
// with a live-updating display.
//
// Usage:
//
//	worldtime                    # Show 10 default cities
//	worldtime london tokyo       # Show specific cities
//	worldtime "new york" paris   # Use quotes for multi-word city names
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ryjiang/worldtime/internal/cities"
	"github.com/ryjiang/worldtime/internal/display"
)

func main() {
	var cityList []cities.City

	args := os.Args[1:]

	if len(args) == 0 {
		cityList = cities.DefaultCities()
	} else {
		var err error
		cityList, err = cities.ParseCities(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Set up context with signal handling for clean exit
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		cancel()
	}()

	display.Run(ctx, cityList)
}
