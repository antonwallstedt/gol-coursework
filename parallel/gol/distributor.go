package gol

import (
	"fmt"

	"uk.ac.bris.cs/gameoflife/util"
)

const ALIVE = 255
const DEAD = 0

type workerWorld struct {
	data [][]byte
}

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioInput    <-chan uint8
	ioOutput   chan<- uint8
}

/* mod : Calculates the remainder of a given number x when divided by m.
Doesn't allow negative numbers so m is added to x before computing mod. */
func mod(x, m int) int {
	return (x + m) % m
}

/* calculateNeighbours : Counts the number of alive neighbours around a given cell.
Does it in a closed domain, i.e. the top-most pixels are connected to the bottom,
and left-most pixels are connected to the right-most pixels and vice versa. */
func calculateNeighbours(x, y int, world [][]byte) int {
	neighbours := 0
	height := len(world)
	width := len(world[0])
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if i != 0 || j != 0 {
				if world[mod(y+i, height)][mod(x+j, width)] == ALIVE {
					neighbours++
				}
			}
		}
	}
	return neighbours
}

/* calculateNextState : Completes one whole evolution of the game and returns the new state */
func calculateNextState(p Params, world [][]byte) [][]byte {
	height := len(world)
	width := p.ImageWidth
	newWorld := makeWorld(height, width)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			neighbours := calculateNeighbours(x, y, world)
			if world[y][x] == ALIVE {
				if neighbours == 2 || neighbours == 3 {
					newWorld[y][x] = ALIVE
				} else {
					newWorld[y][x] = DEAD
				}
			} else {
				if neighbours == 3 {
					newWorld[y][x] = ALIVE
				} else {
					newWorld[y][x] = DEAD
				}
			}
		}
	}
	return newWorld
}

/* calculateAliveCells : Locates alive cells in a given world and returns a slice containing coordinates to those cells */
func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	aliveCells := []util.Cell{}
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == ALIVE {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	return aliveCells
}

/* makeWorld : Creates a 2D world from given height and width */
func makeWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

/* makeWorkerWorld : Create worker worlds from a given world.
This will split a world up into a given number of parts, pad them out with one extra row at the top and one at the bottom, and then fill
those rows with the pixel values of the bottom and top most rows of the other parts surrounding the current part being worked on. */
func makeWorkerWorlds(p Params, world [][]byte) []workerWorld {
	workerWorlds := []workerWorld{}

	// Divide the image height into multiple parts, as image sizes and number of threads are integer values, the division will be floored.
	// The worker heights corresponding to each thread to be run will be stored in an array, this way for each worker we can load in its
	// worker height, and if any one of the worker heights need to be modified (e.g. to account for uneven division) this can easily be done.
	workerHeights := make([]int, p.Threads)
	for i := range workerHeights {
		workerHeights[i] = p.ImageHeight / p.Threads
	}

	// If the number of threads doesn't evenly divide the image height, we need to have one worker working on a different image
	// height to account for the flooring, this height will be +1 in size relative to the other parts, so we set the worker height
	// corresponding to the final worker thread to be +1 in height.
	remainder := p.ImageHeight % p.Threads
	if remainder != 0 {
		for i := 0; i < remainder; i++ {
			workerHeights[i]++
		}
	}

	/* Split picture up into a number of parts corresponding to the number of threads that will be run, pad them out with an extra two rows
	(top and bottom), to account for the halo rows so that when the next state is calculated for each part, it will take into account the
	adjacent pixels that were disconnected when splitting. */
	currHeight := 0
	for i := 0; i < p.Threads; i++ {
		newWorld := makeWorld(workerHeights[i]+2, p.ImageWidth)
		currHeight += workerHeights[i]

		// Top most pixels of the first part must be connected with the bottom most pixels of the last part,
		// and bottom most pixels must correspond to top most pixels of second part, so fill out padding with those values
		if i == 0 {
			newWorld[0] = world[p.ImageHeight-1]          // top most pixels
			newWorld[len(newWorld)-1] = world[currHeight] // bottom most pixels
			for y := 1; y <= workerHeights[i]; y++ {      // remainding values
				newWorld[y] = world[y-1]
			}
			workerWorlds = append(workerWorlds, workerWorld{data: newWorld})
		} else if i == p.Threads-1 {
			newWorld[0] = world[currHeight-workerHeights[i]-1] // top most pixels
			newWorld[len(newWorld)-1] = world[0]               // bottom most pixels
			for y := 1; y <= workerHeights[i]; y++ {           // remainding values
				newWorld[y] = world[i*workerHeights[i]+y-1]
			}
			workerWorlds = append(workerWorlds, workerWorld{data: newWorld})
		} else {
			newWorld[0] = world[currHeight-workerHeights[i]-1] // top most pixels
			newWorld[len(newWorld)-1] = world[currHeight]      // bottom most pixels
			for y := 1; y <= workerHeights[i]; y++ {           // remainding values
				newWorld[y] = world[i*workerHeights[i]+y-1]
			}
			workerWorlds = append(workerWorlds, workerWorld{data: newWorld})
		}
	}
	return workerWorlds
}

/* worker : Takes in part of a world, calculates next step and sends results down a channel for re-assembly */
func worker(p Params, workerWorld [][]byte, c chan<- [][]byte) {
	workerWorldHeight := len(workerWorld)
	newWorld := workerWorld[1:(workerWorldHeight - 1)] // without padding that was added in worker world
	for turn := 0; turn < p.Turns; turn++ {
		newWorld = calculateNextState(p, workerWorld)[1:(workerWorldHeight - 1)] // remove extra padded rows
	}
	c <- newWorld
}

/* makeImmutableWorld : Takes a world and returns an immutable version of it */
func makeImmutableWorld(world [][]byte) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return world[y][x]
	}
}

/* distributor : Divides the work between workers and interacts with other goroutines. */
func distributor(p Params, c distributorChannels) {

	// Request input from and send filename to IO
	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%vx%v", p.ImageHeight, p.ImageWidth)

	// TODO: For all initially alive cells send a CellFlipped Event.
	world := makeWorld(p.ImageHeight, p.ImageWidth)

	// Load in cells into our new world from a given file
	for y := range world {
		for x := range world {
			world[y][x] = <-c.ioInput
		}
	}

	// Create channels to send out results of image parts processed by each worker
	out := make([]chan [][]byte, p.Threads)
	for i := range out {
		out[i] = make(chan [][]byte)
	}

		}
	}
	TurnComplete.CompletedTurns = turn
	c.events <- TurnComplete
	FinalTurnComplete.Alive = listCell
	FinalTurnComplete.CompletedTurns = turn
	c.events <- FinalTurnComplete
	// TODO: Execute all turns of the Game of Life.
	// TODO: Send correct Events when required, e.g. CellFlipped, TurnComplete and FinalTurnComplete.
	//		 See event.go for a list of all events.
	if p.Threads == 1 { // if there's only one thread evolve game normally
		for turn := 0; turn < p.Turns; turn++ {
			world = calculateNextState(p, world)
		}
	} else { // else split image up and process each part with an individual worker
		workerWorlds := makeWorkerWorlds(p, world)
		for i, workerWorld := range workerWorlds {
			go worker(p, workerWorld.data, out[i])
		}

		// Assemble world again
		newWorld := makeWorld(0, 0)
		for i := 0; i < p.Threads; i++ {
			part := <-out[i]
			newWorld = append(newWorld, part...)
		}
		world = newWorld
	}

	// TODO: remove this and come up with another way of getting the value of turns from each worker back, as the turns loop
	// 		 is now running inside each individual worker
	turn := 0
	for turn < p.Turns {
		// immutableWorld := makeImmutableWorld(world) – TODO: see if we can make use of immutable data structures instead
		turn++
	}
	aliveCells := calculateAliveCells(p, world)
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: aliveCells}

	// Make sure that the IO has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
