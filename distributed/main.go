package main

import (
	"flag"
	"fmt"
	"runtime"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/sdl"
)

// main is the function called when starting Game of Life ff with 'go run .'
func main() {
	runtime.LockOSThread()
	var params gol.Params

	flag.IntVar(
		&params.Threads,
		"t",
		8,
		"Specify the number of worker threads to use. Defaults to 8.")

	flag.IntVar(
		&params.ImageWidth,
		"w",
		512,
		"Specify the width of the image. Defaults to 512.")

	flag.IntVar(
		&params.ImageHeight,
		"h",
		512,
		"Specify the height of the image. Defaults to 512.")

	flag.IntVar(
		&params.Turns,
		"turns",
		10000000000,
		"Specify the number of turns to process. Defaults to 10000000000.")

	flag.BoolVar(
		&params.Reconnect,
		"reconnect",
		false,
		"Specify if controller should try to reconnect to an already running engine. Defaults to false.")

	flag.IntVar(
		&params.Threads,
		"workers",
		2,
		"Specify the number of workers to use. Defaults to 2.",
	)

	flag.Parse()

	fmt.Println("Threads:", params.Threads)
	fmt.Println("Width:", params.ImageWidth)
	fmt.Println("Height:", params.ImageHeight)
	fmt.Println("Reconnect:", params.Reconnect)

	keyPresses := make(chan rune, 10)
	events := make(chan gol.Event, 1000)

	gol.Run(params, events, keyPresses)
	sdl.Start(params, events, keyPresses)
}
