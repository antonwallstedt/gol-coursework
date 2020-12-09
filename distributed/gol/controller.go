package gol

import (
	"flag"
	"fmt"
	"net/rpc"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

const (
	ALIVE = 255
	DEAD  = 0
)

type Work struct {
	World [][]byte
	Turn  int
}

type controllerChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

func makeWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

func calculateAliveCells(world [][]byte) []util.Cell {
	aliveCells := []util.Cell{}
	height := len(world)
	width := len(world[0])
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if world[y][x] == ALIVE {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	return aliveCells
}

// Requests the engine to compute the Game of Life for a given number of turns and on a given world
func requestGameOfLife(client rpc.Client, world [][]byte, turns int) Work {
	request := stubs.Request{World: world, Turns: turns}
	response := new(stubs.Response)
	client.Call(stubs.GameOfLifeHandler, request, response)
	return Work{World: response.World, Turn: response.Turn}
}

func requestAliveCells(client rpc.Client) int {
	request := stubs.RequestAliveCells{}
	response := new(stubs.ResponseAliveCells)
	client.Call(stubs.AliveCellsHandler, request, response)
	return response.NumAliveCells
}

func controller(p Params, c controllerChannels) {
	// Request IO to read image file
	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%dx%d", p.ImageHeight, p.ImageWidth)

	// Dial server
	var serverIP string
	if flag.Lookup("server") != nil {
		serverIP = flag.Lookup("server").Value.String()
	} else {
		serverIP = "127.0.0.1:8030"
	}
	client, _ := rpc.Dial("tcp", serverIP)
	defer client.Close()

	// Load world in
	world := makeWorld(p.ImageHeight, p.ImageWidth)
	for y := range world {
		for x := range world {
			world[y][x] = <-c.ioInput
		}
	}

	var numAliveCells int
	ticker := time.NewTicker(2 * time.Second)
	go func() {
		select {
		case <-ticker.C:
			numAliveCells = requestAliveCells(*client)
			c.events <- AliveCellsCount{CompletedTurns: 0, CellsCount: numAliveCells}
		default:
		}
	}()

	// Make call to server to process game
	resultWork := requestGameOfLife(*client, world, p.Turns)

	// Calculate alive cells
	c.events <- FinalTurnComplete{CompletedTurns: resultWork.Turn, Alive: calculateAliveCells(resultWork.World)}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{resultWork.Turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)

}
