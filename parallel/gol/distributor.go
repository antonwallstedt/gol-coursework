package gol

import (
	"fmt"

	"uk.ac.bris.cs/gameoflife/util"
)

// ALIVE : byte value for alive cells
const ALIVE = 255

// DEAD : byte value for dead cells
const DEAD = 0

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

/* OLD FUNCTION
func calculateNeighbours(p Params, x, y int, world [][]byte) int {
	neighbours := 0
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if i != 0 || j != 0 {
				if world[mod(y+i, p.ImageHeight)][mod(x+j, p.ImageWidth)] == ALIVE {
					neighbours++
				}
			}
		}
	}
	return neighbours
} */

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

/* OLD FUNCTION
func calculateNextState(p Params, world [][]byte) [][]byte {
	newWorld := make([][]byte, p.ImageHeight)
	for i := range newWorld {
		newWorld[i] = make([]byte, p.ImageWidth)
	}
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			neighbours := calculateNeighbours(p, x, y, world)
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
} */

/* calculateNextState : completes one whole evolution of the game and returns the new state */
func calculateNextState(startY, startX, endY, endX int, world [][]byte) [][]byte {
	height := len(world)
	width := len(world[0])
	newWorld := makeWorld(height, width)
	for y := startY; y <= endY; y++ {
		for x := startX; x < endX; x++ {
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

/* calculateAliveCells : locates alive cells in a given world and returns a slice containing coordinates to those cells */
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

/* worker : takes in part of a world, calculates next step and sends results down a channel for re-assembly */
func worker(startY, startX, endY, endX int, world [][]byte, c chan<- [][]byte) {
	newWorld := calculateNextState(startY, startX, endY, endX, world)
	c <- newWorld
}

/* makeWorld : creates a 2x2 world from given height and width */
func makeWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

/* makeImmutableWorld : takes a world and returns an immutable version of it */
func makeImmutableWorld(world [][]byte) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return world[y][x]
	}
}

/* distributor : Divides the work between workers and interacts with other goroutines. */
func distributor(p Params, c distributorChannels) {

	/* 	Explanatory comment
	*
	* We start with sending the ioInput command (which is an enum with a specific int value) down the command channel.
	* startIO (in gol.go line 34) will start the IO goroutine, in which a never-ending for loop is running,
	* this loop is used in conjunction with a select statement (in io.go line 124 - 143), which means whenever a command is
	* sent down the ioCommand channel, this select statement will pick up that command and go into one of the three cases.
	*
	* In the case of ioInput, the readPgmImage() function inside io.go will be waiting for a filename, that's why
	* we send the filename down the ioFilename channel, and the filename channel in the distributor is send only.
	*
	 */

	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%vx%v", p.ImageHeight, p.ImageWidth)

	// TODO: Create a 2D slice to store the world.
	// TODO: For all initially alive cells send a CellFlipped Event.
	world := makeWorld(p.ImageHeight, p.ImageWidth)

	// Load in cells into our new world from a given file
	for y := range world {
		for x := range world {
			world[y][x] = <-c.ioInput
		}
	}

	// TODO: Execute all turns of the Game of Life.
	// TODO: Send correct Events when required, e.g. CellFlipped, TurnComplete and FinalTurnComplete.
	//		 See event.go for a list of all events.
	turn := 0
	for turn < p.Turns {
		// immutableWorld := makeImmutableWorld(world) – TODO: see if we can make use of immutable data structures instead

		// Create channels to send out processed image parts
		out := make([]chan [][]byte, p.Threads)
		for i := range out {
			out[i] = make(chan [][]byte)
		}

		// Divide the image height into multiple parts, as image sizes and number of threads are integer values, the division will be floored.
		// Therefore, if the number of threads doesn't evenly divide the image height, we need to have one worker working on a different image
		// height to account for the flooring, this height will be +1 in size relative to the other parts.
		workerHeight := p.ImageHeight / p.Threads
		if p.ImageHeight%p.Threads != 0 {
			workerHeightPadded := p.ImageHeight/p.Threads + 1
			finalRowNum := p.Threads - 1
			for i := 0; i < p.Threads-1; i++ {
				go worker(i*workerHeight, 0, (i+1)*workerHeight, p.ImageWidth, world, out[i])
			}
			go worker(finalRowNum*workerHeightPadded, 0, (finalRowNum+1)*workerHeightPadded, p.ImageWidth, world, out[finalRowNum])
		}

		// TODO: Let each worker run on their individual part
		for i := 0; i < p.Threads; i++ {
			go worker(i*workerHeight, 0, (i+1)*workerHeight, p.ImageWidth, world, out[i])
		}

		// TODO: Assemble world again
		newWorld := makeWorld(0, 0)
		for i := 0; i < p.Threads; i++ {
			part := <-out[i]
			newWorld = append(newWorld, part...)
		}

		fmt.Println(len(world))
		fmt.Println(len(newWorld))
		world = newWorld

		/*startY := 0
		startX := 0
		endY := len(world)
		endX := len(world[0])
		world = calculateNextState(startY, startX, endY, endX, world)*/

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
