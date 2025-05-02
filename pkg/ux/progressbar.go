// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"fmt"
	"time"

	ansi "github.com/k0kubun/go-ansi"
	progressbar "github.com/schollz/progressbar/v3"
)

func TimedProgressBar(
	duration time.Duration,
	title string,
	extraSteps int,
) (*progressbar.ProgressBar, error) {
	const steps = 1000
	stepDuration := duration / steps
	bar := progressbar.NewOptions(steps+extraSteps,
		progressbar.OptionSetWriter(ansi.NewAnsiStdout()),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetElapsedTime(false),
		progressbar.OptionSetWidth(15),
		progressbar.OptionSetDescription(title),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))
	for i := 0; i < steps; i++ {
		if err := bar.Add(1); err != nil {
			return nil, err
		}
		time.Sleep(stepDuration)
	}
	if extraSteps == 0 {
		fmt.Println()
	}
	return bar, nil
}

func ExtraStepExecuted(bar *progressbar.ProgressBar) error {
	return bar.Add(1)
}
