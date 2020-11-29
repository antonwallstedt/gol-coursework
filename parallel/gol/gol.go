package gol

// Test comment by Anton
// Test comment by ly
// Git is finally working :)

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {
	ioFileName := make(chan string)
	ioInput := make(chan uint8)
	ioOutput := make(chan uint8)
	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)

	distributorChannels := distributorChannels{
		events,
		ioCommand,
		ioIdle,
		ioFileName,
		ioInput,
		ioOutput,
	}
	go distributor(p, distributorChannels, keyPresses)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: ioFileName,
		output:   ioOutput,
		input:    ioInput,
	}

	go startIo(p, ioChannels)
}
