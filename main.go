package main

import (
	"context"
	"fmt"
	"github.com/gookit/color"
	"github.com/wagoodman/go-progress"
	"github.com/wagoodman/jotframe/pkg/frame"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

const downloadColor = color.FgRed
const scanColor = color.FgGreen

const maxBarWidth = 50
const statusSet = SpinnerArrowSet

const (
	LiteTheme SimpleTheme = iota
	LiteSquashTheme
	HeavyTheme
	HeavySquashTheme
	ReallyHeavySquashTheme
	HeavyNoBarTheme
)

const (
	emptyPosition SimplePosition = iota
	fullPosition
	leftCapPosition
	rightCapPosition
)
const completedStatus = "✔" // "●"

type Simple struct {
	width     int
	charSet   []string
	doneColor color.RGBColor
	todoColor color.RGBColor
}

type SimplePosition int

type SimpleTheme int

var (
	ColorCompleted      = color.HEX("#fcba03")
	ColorTodo           = color.HEX("#777777")
	auxInfoFormat       = color.HEX("#777777")
	statusTitleTemplate = fmt.Sprintf(" %%s %%-%ds ", 31)
)

var lookup = map[SimpleTheme][]string{
	LiteTheme:              {" ", "─", "├", "┤"},
	LiteSquashTheme:        {" ", "─", "▕", "▏"},
	HeavyTheme:             {"━", "━", "┝", "┥"},
	HeavySquashTheme:       {"━", "━", "▕", "▏"},
	ReallyHeavySquashTheme: {"━", "━", "▐", "▌"},
	HeavyNoBarTheme:        {"━", "━", "", ""},
}

//ProgressFormat function to calculate and stringify the progress bar
func (s Simple) ProgressFormat(p progress.Progress) (string, error) {
	completedRatio := p.Ratio()
	if completedRatio < 0 {
		completedRatio = 0
	}
	completedCount := int(completedRatio * float64(s.width))
	todoCount := s.width - completedCount
	if todoCount < 0 {
		todoCount = 0
	}

	completedSection := s.doneColor.Sprint(strings.Repeat(s.charSet[fullPosition], completedCount))
	todoSection := s.todoColor.Sprint(strings.Repeat(s.charSet[fullPosition], todoCount))

	return s.charSet[leftCapPosition] + completedSection + todoSection + s.charSet[rightCapPosition], nil
}

//downloadProgress function to demonstrate progress bar UI
func downloadProgress(wg *sync.WaitGroup, fr *frame.Frame) {
	wg.Add(1)

	line, err := fr.Append()
	if err != nil {
		log.Fatalf("error obtaining line from append %v", err)
	}

	// set size of 1000
	cp := progress.NewSizedWriter(1000)

	//create aggregator to aggregate the value
	aggregateProgress := progress.NewAggregator(progress.NormalizeStrategy, cp)

	//wrap aggregator into StagedProggresable
	var prog = progress.StagedProgressable(&struct {
		progress.Stager
		*progress.Aggregator
	}{
		Stager:     progress.Stager(&progress.Stage{}),
		Aggregator: aggregateProgress,
	})

	//stream it to make it easy to print
	stream := progress.Stream(context.Background(), prog, 25*time.Millisecond)

	//setup spinner
	formatter, spinner := setupSpinner()
	title := downloadColor.Sprint("Download progress")

	formatFn := func(p progress.Progress) {
		progStr, err := formatter.ProgressFormat(p)
		spin := color.Magenta.Sprint(spinner.Next())
		if err != nil {
			_, _ = io.WriteString(line, fmt.Sprintf("Error: %+v", err))
		} else {
			auxInfo := auxInfoFormat.Sprintf("[%s]", prog.Stage())
			_, _ = io.WriteString(line, fmt.Sprintf(statusTitleTemplate+"%s %s", spin, title, progStr, auxInfo))
		}
	}

	//goroutine to loop through value
	go func() {
		for i := 0; i < 1000; i += 1 {
			//write to NewSizedWriter which will be read by aggregator
			cp.Write([]byte(string(i)))
			time.Sleep(1 * time.Millisecond)
		}

		//call SetComplete() once everything is done
		cp.SetComplete()
	}()

	//goroutine to process the stream and print it out using formatFn() function
	go func() {
		defer wg.Done()

		for p := range stream {
			formatFn(p)
		}
		spin := color.Green.Sprint(completedStatus)
		title = downloadColor.Sprint("Downloaded complete")
		_, _ = io.WriteString(line, fmt.Sprintf(statusTitleTemplate, spin, title))
	}()

}

//scanningImage demonstrate the counter/gauge UI
func scanningImage(wg *sync.WaitGroup, fr *frame.Frame) {
	wg.Add(1)

	//get a line to put the ui on
	line, err := fr.Append()
	if err != nil {
		log.Fatalf("error obtaining line from append %v", err)
	}

	_, spinner := setupSpinner()

	//setup title with scanColor
	title := scanColor.Sprint("Scanning image...")

	//function to display the counter pass in val parameter
	formatFn := func(val int) {
		//get the spinner string
		spin := color.Magenta.Sprint(spinner.Next())

		//get the text to be displayed
		auxInfo := auxInfoFormat.Sprintf("[vulnerabilities %d]", val)

		//print all to screen
		_, _ = io.WriteString(line, fmt.Sprintf(statusTitleTemplate+"%s", spin, title, auxInfo))
	}

	//goroutine to loop through counter that will be printed using the formatFn(..) function
	go func() {
		defer wg.Done()
		formatFn(0)
		v := 1
		for ; v < 550; v++ {
			formatFn(v)
			time.Sleep(2 * time.Millisecond)
		}

		// once it's done print something else
		spin := color.Green.Sprint(completedStatus)
		title = scanColor.Sprint("Scanned image")
		auxInfo := auxInfoFormat.Sprintf("[%d vulnerabilities]", v)
		_, _ = io.WriteString(line, fmt.Sprintf(statusTitleTemplate+"%s", spin, title, auxInfo))
	}()
}

func NewSimpleWithTheme(width int, theme SimpleTheme, doneHexColor, todoHexColor color.RGBColor) Simple {
	return Simple{
		width:     width,
		charSet:   lookup[theme],
		doneColor: doneHexColor,
		todoColor: todoHexColor,
	}
}

//setupSpinner to setup formatter and spinner
func setupSpinner() (Simple, *Spinner) {
	width, _ := frame.GetTerminalSize()
	barWidth := int(0.25 * float64(width))
	if barWidth > maxBarWidth {
		barWidth = maxBarWidth
	}
	formatter := NewSimpleWithTheme(barWidth, LiteTheme, ColorCompleted, ColorTodo)
	spinner := NewSpinner(statusSet)

	return formatter, &spinner
}

//setupScreen to setup frame based on screen configuration
func setupScreen(output *os.File) *frame.Frame {
	config := frame.Config{
		PositionPolicy: frame.PolicyFloatForward,
		Output:         output,
	}

	fr, err := frame.New(config)
	if err != nil {
		log.Fatalf("failed to create screen object: %+v", err)
		return nil
	}
	return fr
}

func main() {
	output := os.Stdout
	var wg = &sync.WaitGroup{}

	// hide cursor
	_, _ = fmt.Fprint(output, "\x1b[?25l")

	// show cursor
	defer fmt.Fprint(output, "\x1b[?25h")

	fr := setupScreen(output)

	defer func() {
		// wait until all tasks are done
		wg.Wait()

		// close the opened frame
		fr.Close()

		frame.Close()
	}()

	scanningImage(wg, fr)
	downloadProgress(wg, fr)
}
