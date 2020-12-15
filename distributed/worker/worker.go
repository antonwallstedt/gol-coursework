package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/stubs"
)

// Global worker world
var globalWorkerWorld [][]byte

const (
	// ALIVE : pixel value for alive cells
	ALIVE = 255

	// DEAD : pixel value for dead cells
	DEAD = 0
)

type Worker struct{}

func makeWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

func mod(x, m int) int {
	return (x + m) % m
}

// Calculates the number of alive neighbours around a given cell
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

// Computes one evolution of the Game of Life
func calculateNextState(world [][]byte) [][]byte {
	height := len(world)
	width := len(world[0])
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

// StartWorker : starts the worker by receiving the worker world from the RPC request and sends back halo rows
func (w *Worker) StartWorker(req stubs.RequestStartWorker, res *stubs.ResponseRows) (err error) {
	fmt.Println("Worker started")
	globalWorkerWorld = req.WorkerWorld
	globalWorkerWorld = calculateNextState(globalWorkerWorld)
	res.TopRow = globalWorkerWorld[1]
	res.BottomRow = globalWorkerWorld[len(globalWorkerWorld)-1]
	return
}

// CalculateNextState : calculates the next state from given halo rows
func (w *Worker) CalculateNextState(req stubs.RequestNextState, res *stubs.ResponseRows) (err error) {
	topRow := req.TopRow
	bottomRow := req.BottomRow
	globalWorkerWorld[0] = topRow
	globalWorkerWorld[len(globalWorkerWorld)-1] = bottomRow
	globalWorkerWorld = calculateNextState(globalWorkerWorld)
	res.TopRow = globalWorkerWorld[1]
	res.BottomRow = globalWorkerWorld[len(globalWorkerWorld)-1]
	fmt.Println("Next state calculated")
	return
}

// GetResult : Gets the result of this worker and sends it back, excluding the extra top and bottom rows
func (w *Worker) GetResult(req stubs.RequestWorkerResult, res *stubs.ResponseWorkerResult) (err error) {
	globalWorkerWorldPart := globalWorkerWorld[1 : len(globalWorkerWorld)-1]
	res.WorkerWorldPart = globalWorkerWorldPart
	return
}

func main() {
	pAddr := flag.String("port", "8050", "Port to listen on")
	flag.Parse()
	rpc.Register(&Worker{})
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println(err)
	}
	defer listener.Close()
	rpc.Accept(listener)
}
