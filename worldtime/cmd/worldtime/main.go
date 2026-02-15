package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ryjiang/agent-platform/tools/worldtime/internal/clock"
)

func main() {
	cities := clock.DefaultCities()

	// Handle Ctrl+C gracefully
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Initial render
	render(cities)

	for {
		select {
		case <-ticker.C:
			render(cities)
		case <-sig:
			fmt.Print("\033[?25h") // show cursor
			fmt.Println("\n  Goodbye!")
			os.Exit(0)
		}
	}
}

func render(cities []clock.City) {
	now := time.Now()
	local := clock.GetLocalTime(now)

	var cityTimes []clock.CityTime
	for _, c := range cities {
		ct, err := clock.GetCityTime(c, now)
		if err != nil {
			continue
		}
		cityTimes = append(cityTimes, ct)
	}

	fmt.Print("\033[?25l") // hide cursor
	fmt.Print(clock.Render(local, cityTimes))
}
