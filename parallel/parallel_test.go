package main

import (
	"fmt"
	"os"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

const (
	turns       = 250
	imageHeight = 512
	imageWidth  = 512
)

func Benchmark(b *testing.B) {
	params := gol.Params{Turns: turns, ImageWidth: imageWidth, ImageHeight: imageHeight}
	for threads := 1; threads <= 16; threads++ {
		params.Threads = threads
		testName := fmt.Sprintf("%dx%dx%d-%d", params.ImageWidth, params.ImageHeight, params.Turns, params.Threads)
		os.Stdout = nil
		for i := 0; i < b.N; i++ {
			b.Run(testName, func(b *testing.B) {
				b.StartTimer()
				events := make(chan gol.Event)
				gol.Run(params, events, nil)
				for event := range events {
					switch event.(type) {
					case gol.FinalTurnComplete:
						break
					}
				}
				b.StopTimer()
			})
		}

	}
}
