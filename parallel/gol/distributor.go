package gol

import (
	"fmt"

	"uk.ac.bris.cs/gameoflife/util"
)

const Alive = 255
const Dead = 0

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFileName chan<- string
	ioInput    <-chan uint8
	ioOutput   chan<- uint8
}

func aliveNeighbour(p Params, y, x int, world [][]byte) int {

	var a int
	var b int

	prevX := x - 1
	aftX := x + 1
	prevY := y - 1
	aftY := y + 1
	if x == 0 {
		prevX = p.ImageWidth - 1
	}
	if x == p.ImageWidth-1 {
		aftX = 0
	}
	if y == 0 {
		prevY = p.ImageHeight - 1
	}
	if y == p.ImageHeight-1 {
		aftY = 0
	}

	b = int(world[y][prevX]) + int(world[y][aftX]) + int(world[prevY][prevX]) + int(world[prevY][x]) + int(world[prevY][aftX]) + int(world[aftY][aftX]) + int(world[aftY][prevX]) + int(world[aftY][x])

	a = b / 255

	return a
}
func swappinng(oldWorld [][]byte, newWorld [][]byte) {
	x := oldWorld
	oldWorld = newWorld
	newWorld = x
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	c.ioCommand <- ioInput
	c.ioFileName <- fmt.Sprintf("%vx%v", p.ImageHeight, p.ImageWidth)
	var Cell util.Cell
	var listCell []util.Cell
	var CellFlip CellFlipped
	var TurnComplete TurnComplete
	var FinalTurnComplete FinalTurnComplete

	// TODO: Create a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}
	newWorld := make([][]byte, p.ImageHeight)
	for i := range newWorld {
		newWorld[i] = make([]byte, p.ImageWidth)
	}
	// TODO: For all initially alive cells send a CellFlipped Event.
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			input := <-c.ioInput
			world[y][x] = input
		}
	}
	turn := 0
	for turn < p.Turns {
		for y := 0; y < p.ImageHeight; y++ {
			for x := 0; x < p.ImageWidth; x++ {
				a := aliveNeighbour(p, y, x, world)
				if world[y][x] == Alive {
					if a == 2 || a == 3 {
						newWorld[y][x] = Alive
						// listCell = append(listCell, util.Cell{X: x, Y: y})

					} else {
						newWorld[y][x] = Dead
						Cell.X = x
						Cell.Y = y
						CellFlip.Cell = Cell
						CellFlip.CompletedTurns = turn
						c.events <- CellFlip

					}
				} else {
					if a == 3 {
						newWorld[y][x] = Alive
						Cell.X = x
						Cell.Y = y
						CellFlip.Cell = Cell
						CellFlip.CompletedTurns = turn
						c.events <- CellFlip
						// listCell = append(listCell, util.Cell{X: x, Y: y})

					} else {
						newWorld[y][x] = Dead
					}
				}
			}
		}

		turn++
		x := world
		world = newWorld
		newWorld = x

		TurnComplete.CompletedTurns = turn
		c.events <- TurnComplete

	}

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == Alive {
				listCell = append(listCell, util.Cell{X: x, Y: y})
			}
		}
	}
	FinalTurnComplete.Alive = listCell
	FinalTurnComplete.CompletedTurns = turn
	c.events <- FinalTurnComplete

	// TODO: Execute all turns of the Game of Life.
	// TODO: Send correct Events when required, e.g. CellFlipped, TurnComplete and FinalTurnComplete.
	//		 See event.go for a list of all events.

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)

}
