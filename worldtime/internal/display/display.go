// Package display handles terminal rendering of world time with live updates.
package display

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ryjiang/worldtime/internal/cities"
)

const (
	// ANSI escape sequences
	clearScreen = "\033[2J"
	cursorHome  = "\033[H"
	bold        = "\033[1m"
	reset       = "\033[0m"
	dim         = "\033[2m"
	cyan        = "\033[36m"
	yellow      = "\033[33m"
)

// Render writes the formatted time display for the given cities to the writer.
func Render(w io.Writer, cityList []cities.City, now time.Time) {
	fmt.Fprint(w, clearScreen+cursorHome)
	fmt.Fprintf(w, "%s%s 🌍 World Time %s%s\n", bold, cyan, reset, dim+now.UTC().Format("(UTC 2006-01-02 15:04:05)")+reset)
	fmt.Fprintln(w, strings.Repeat("─", 52))

	for _, c := range cityList {
		loc, err := time.LoadLocation(c.Timezone)
		if err != nil {
			fmt.Fprintf(w, "  %-20s %serror loading timezone%s\n", c.Name, dim, reset)
			continue
		}
		cityTime := now.In(loc)
		zone, _ := cityTime.Zone()
		fmt.Fprintf(w, "  %s%-20s%s %s%s%s %s%s%s\n",
			yellow, c.Name, reset,
			bold, cityTime.Format("Mon 2006-01-02 15:04:05"), reset,
			dim, zone, reset,
		)
	}

	fmt.Fprintln(w, strings.Repeat("─", 52))
	fmt.Fprintf(w, "%sPress Ctrl+C to exit%s\n", dim, reset)
}

// Run starts a live-updating display loop that refreshes every second.
// It blocks until the context is cancelled.
func Run(ctx context.Context, cityList []cities.City) {
	// Initial render
	Render(os.Stdout, cityList, time.Now())

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Clear screen one final time and show exit message
			fmt.Fprint(os.Stdout, clearScreen+cursorHome)
			fmt.Println("Goodbye!")
			return
		case t := <-ticker.C:
			Render(os.Stdout, cityList, t)
		}
	}
}
