// Command vulpine-mark connects to a CDP browser, annotates the active
// page with numbered labels over interactive elements, and writes the
// annotated PNG plus element map to disk.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/PopcornDev1/vulpine-mark/pkg/vulpinemark"
)

// savedResult is the on-disk form of a vulpinemark.Result. Only the
// fields needed to rehydrate a Result for --diff / --save-result are
// persisted; the PNG image itself is not round-tripped.
type savedResult struct {
	Elements map[string]vulpinemark.Element `json:"elements"`
	Labels   []string                       `json:"labels"`
	Clusters []vulpinemark.Cluster          `json:"clusters"`
}

func main() {
	var (
		cdp         = flag.String("cdp", "http://localhost:9222", "CDP endpoint (http://host:port or ws://...)")
		outputPNG   = flag.String("output", "annotated.png", "Path for the annotated PNG")
		outputJSON  = flag.String("json", "", "Optional path for the element map JSON")
		outputSVG   = flag.String("svg", "", "Optional path for an SVG overlay composite-able onto the screenshot")
		fullPage    = flag.Bool("full-page", false, "Annotate the full scrollable page instead of just the viewport")
		palette     = flag.String("palette", "default", "Color palette: default, high-contrast, monochrome, colorblind")
		clustered   = flag.Bool("clustered", false, "Group repeated similar elements under cluster labels")
		diffPath    = flag.String("diff", "", "Path to a previous --save-result JSON; annotate only changed elements")
		savePath    = flag.String("save-result", "", "Path to save the result as JSON for later --diff use")
		maxElements = flag.Int("max-elements", 0, "Maximum number of elements to label (lowest confidence dropped first). 0 disables the cap.")
		quiet       = flag.Bool("quiet", false, "Suppress progress output")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mark, err := vulpinemark.New(ctx, *cdp)
	if err != nil {
		fail("connect: %v", err)
	}
	defer mark.Close()

	pal, err := vulpinemark.PaletteByName(*palette)
	if err != nil {
		fail("%v", err)
	}
	mark.SetPalette(pal)

	if *maxElements > 0 {
		mark.SetMaxElements(*maxElements)
	}

	if !*quiet {
		fmt.Fprintf(os.Stderr, "vulpine-mark: connected to %s\n", *cdp)
	}

	var prev *vulpinemark.Result
	if *diffPath != "" {
		prev, err = loadSavedResult(*diffPath)
		if err != nil {
			fail("load --diff: %v", err)
		}
	}

	var result *vulpinemark.Result
	switch {
	case prev != nil:
		result, err = mark.AnnotateDiff(ctx, prev)
	case *clustered:
		result, err = mark.AnnotateClustered(ctx)
	case *fullPage:
		result, err = mark.AnnotateFullPage(ctx)
	case *outputSVG != "":
		result, err = mark.AnnotateSVG(ctx)
	default:
		result, err = mark.Annotate(ctx)
	}
	if err != nil {
		fail("annotate: %v", err)
	}

	if err := os.WriteFile(*outputPNG, result.Image, 0o644); err != nil {
		fail("write %s: %v", *outputPNG, err)
	}

	if *outputSVG != "" && result.SVG != "" {
		if err := os.WriteFile(*outputSVG, []byte(result.SVG), 0o644); err != nil {
			fail("write %s: %v", *outputSVG, err)
		}
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

	if *savePath != "" {
		sr := savedResult{
			Elements: result.Elements,
			Labels:   result.Labels,
			Clusters: result.Clusters,
		}
		data, err := json.MarshalIndent(sr, "", "  ")
		if err != nil {
			fail("encode save-result: %v", err)
		}
		if err := os.WriteFile(*savePath, data, 0o644); err != nil {
			fail("write %s: %v", *savePath, err)
		}
	}

	if !*quiet {
		fmt.Fprintf(os.Stderr, "vulpine-mark: %d elements labeled, wrote %s",
			len(result.Labels), *outputPNG)
		if *outputJSON != "" {
			fmt.Fprintf(os.Stderr, " + %s", *outputJSON)
		}
		if *outputSVG != "" {
			fmt.Fprintf(os.Stderr, " + %s", *outputSVG)
		}
		if *savePath != "" {
			fmt.Fprintf(os.Stderr, " + %s", *savePath)
		}
		fmt.Fprintln(os.Stderr)
	}
}

func loadSavedResult(path string) (*vulpinemark.Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sr savedResult
	if err := json.Unmarshal(data, &sr); err != nil {
		return nil, err
	}
	return &vulpinemark.Result{
		Elements: sr.Elements,
		Labels:   sr.Labels,
		Clusters: sr.Clusters,
	}, nil
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "vulpine-mark: "+format+"\n", args...)
	os.Exit(1)
}
