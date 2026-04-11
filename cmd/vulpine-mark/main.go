// Command vulpine-mark connects to a CDP browser, annotates the active
// page with numbered labels over interactive elements, and writes the
// annotated PNG plus element map to disk.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/PopcornDev1/vulpine-mark/pkg/vulpinemark"
)

func main() {
	var (
		cdp        = flag.String("cdp", "http://localhost:9222", "CDP endpoint (http://host:port or ws://...)")
		outputPNG  = flag.String("output", "annotated.png", "Path for the annotated PNG")
		outputJSON = flag.String("json", "", "Optional path for the element map JSON")
		fullPage   = flag.Bool("full-page", false, "Annotate the full scrollable page instead of just the viewport")
		quiet      = flag.Bool("quiet", false, "Suppress progress output")
	)
	flag.Parse()

	mark, err := vulpinemark.New(*cdp)
	if err != nil {
		fail("connect: %v", err)
	}
	defer mark.Close()

	if !*quiet {
		fmt.Fprintf(os.Stderr, "vulpine-mark: connected to %s\n", *cdp)
	}

	var result *vulpinemark.Result
	if *fullPage {
		result, err = mark.AnnotateFullPage()
	} else {
		result, err = mark.Annotate()
	}
	if err != nil {
		fail("annotate: %v", err)
	}

	if err := os.WriteFile(*outputPNG, result.Image, 0o644); err != nil {
		fail("write %s: %v", *outputPNG, err)
	}

	if *outputJSON != "" {
		data, err := json.MarshalIndent(result.Elements, "", "  ")
		if err != nil {
			fail("encode json: %v", err)
		}
		if err := os.WriteFile(*outputJSON, data, 0o644); err != nil {
			fail("write %s: %v", *outputJSON, err)
		}
	}

	if !*quiet {
		fmt.Fprintf(os.Stderr, "vulpine-mark: %d elements labeled, wrote %s",
			len(result.Labels), *outputPNG)
		if *outputJSON != "" {
			fmt.Fprintf(os.Stderr, " + %s", *outputJSON)
		}
		fmt.Fprintln(os.Stderr)
	}
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "vulpine-mark: "+format+"\n", args...)
	os.Exit(1)
}
