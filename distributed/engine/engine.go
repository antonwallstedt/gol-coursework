package main

import (
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
)

// IP addresses for AWS instances for each of the workers to be run on
// TODO: move everything to be run on AWS, change so the same port is being used and
// change all of the IP addresses to match each of the AWS instances.
var workerIPs = map[int]string{
	0: "172.31.80.108:8050",
	1: "172.31.92.159:8050",
	2: "172.31.93.105:8050",
	3: "172.31.89.131:8050",
	4: "172.31.92.21:8050",
	5: "172.31.86.7:8050",
	6: "172.31.86.7:8050",
	7: "172.31.85.232:8050",
}

// Work : used to send work to the engine and to receive work from the engine
type Work struct {
	World [][]byte
	Turn  int
}

// AliveCells : used to receive appropriate values and store them neatly in a struct from RPC calls made by the controller
type AliveCells struct {
	NumAliveCells  int
	CompletedTurns int
}

// TopBottomRows : holds top and bottom rows that are sent back by the workers after they've computed one step
type TopBottomRows struct {
	TopRow    []byte
	BottomRow []byte
}

// WorkerWorld : struct to allow for neat creation of a slice of worlds of type [][]byte
type WorkerWorld struct {
	world [][]byte
}

// WorkerResult : to allow for neat creation of slice containing each worker result and their ID
type WorkerResult struct {
	world    [][]byte
	workerID int
}

const (
	// ALIVE : pixel value for alive cells
	ALIVE = 255

	// DEAD : pixel value for dead cells
	DEAD = 0
)

const (
	requestAliveCells = iota
	requestPgm
	requestPause
	requestStop
	requestStopWorkers
)

var running = false

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

// numAliveCells : gets the number of alive cells from a given world
func numAliveCells(world [][]byte) int {
	aliveCells := 0
	for y := range world {
		for x := range world {
			if world[y][x] == ALIVE {
				aliveCells++
			}
		}
	}
	return aliveCells
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

// makeWorkerHeights : calculate the worker heights for each of the worker and store in an array
func makeWorkerHeights(numWorkers, worldHeight int) []int {
	workerHeights := make([]int, numWorkers)
	defaultHeight := worldHeight / numWorkers // floor division
	for i := range workerHeights {
		workerHeights[i] = defaultHeight
	}

	// In case of uneven division, add remaining height to the last worker
	remainder := worldHeight % numWorkers
	if remainder != 0 {
		workerHeights[len(workerHeights)-1] += remainder
	}
	return workerHeights
}

// buildWorkerWorlds : takes in the number of workers and creates worlds for each of them to work on
func buildWorkerWorlds(workerHeights []int, world [][]byte) []WorkerWorld {
	worldHeight := len(world)
	worldWidth := len(world[0])
	numWorkers := len(workerHeights)
	workerWorlds := []WorkerWorld{}
	currHeight := 0
	for currThread, workerHeight := range workerHeights {
		currHeight += workerHeight
		paddedWorkerHeight := workerHeight + 2 // add extra top and bottom row to account for halo rows
		workerWorld := makeWorld(paddedWorkerHeight, worldWidth)
		if currThread == numWorkers-1 { // bottom most part of the world
			workerWorld[0] = world[(currThread*workerHeights[0]+worldHeight-1)%worldHeight] // set top halo row
			workerWorld[paddedWorkerHeight-1] = world[0]                                    // set bottom halo row
			for y := 1; y < paddedWorkerHeight-1; y++ {                                     // fill in middle
				workerWorld[y] = world[currThread*workerHeights[0]+y-1]
			}
			workerWorlds = append(workerWorlds, WorkerWorld{world: workerWorld})
		} else { // remaining parts
			workerWorld[0] = world[(currThread*workerHeight+worldHeight-1)%worldHeight]
			workerWorld[paddedWorkerHeight-1] = world[((currThread+1)*workerHeight+worldHeight)%worldHeight]
			for y := 1; y < paddedWorkerHeight-1; y++ {
				workerWorld[y] = world[currThread*workerHeight+y-1]
			}
			workerWorlds = append(workerWorlds, WorkerWorld{world: workerWorld})
		}
	}
	return workerWorlds
}

func assembleWorkerParts(workerClients []*rpc.Client, numWorkers int) [][]byte {
	// Store the worker result in a map, where the key is the worker ID of each worker with their corresponding world.
	// Since maps are not ordered, the workerID has to be retrieved from the workers so we are 100% certain the key corresponds
	// to their actual world.
	workerParts := map[int][][]byte{}
	for i := 0; i < numWorkers; i++ {
		workerPartResult := requestWorkerResult(*workerClients[i], numWorkers)
		workerParts[workerPartResult.workerID] = workerPartResult.world
	}

	// Loop through like this rather than using range, as maps are unordered
	workerResult := makeWorld(0, 0)
	for i := 0; i < numWorkers; i++ {
		part := workerParts[i]
		workerResult = append(workerResult, part...)
	}

	return workerResult
}

/* RCP calls */

func requestStartWorker(client rpc.Client, workerWorld [][]byte, workerID int) TopBottomRows {
	request := stubs.RequestStartWorker{WorkerWorld: workerWorld, WorkerID: workerID}
	response := new(stubs.ResponseRows)
	client.Call(stubs.StartWorkerHandler, request, response)
	return TopBottomRows{TopRow: response.TopRow, BottomRow: response.BottomRow}
}

func requestNextState(client rpc.Client, topBottomRows TopBottomRows) TopBottomRows {
	request := stubs.RequestNextState{TopRow: topBottomRows.TopRow, BottomRow: topBottomRows.BottomRow}
	response := new(stubs.ResponseRows)
	client.Call(stubs.NextStateHandler, request, response)
	return TopBottomRows{TopRow: response.TopRow, BottomRow: response.BottomRow}
}

func requestWorkerResult(client rpc.Client, numWorkers int) WorkerResult {
	request := stubs.RequestWorkerResult{NumWorkers: numWorkers}
	response := new(stubs.ResponseWorkerResult)
	client.Call(stubs.WorkerResultHandler, request, response)
	return WorkerResult{world: response.WorkerWorldPart, workerID: response.WorkerID}
}

func requestWorkerPGM(client rpc.Client) WorkerResult {
	request := stubs.RequestPGM{}
	response := new(stubs.ResponseWorkerResult)
	client.Call(stubs.WorkerPGMHandler, request, response)
	return WorkerResult{world: response.WorkerWorldPart, workerID: response.WorkerID}
}

func requestStopWorker(client rpc.Client) {
	request := stubs.RequestStopWorker{}
	response := new(stubs.ResponseStopWorker)
	client.Call(stubs.StopWorkerHandler, request, response)
	return
}

// Evolves the Game of Life for a given number of turns and a given world
func gameOfLife(numWorkers, turns int, world [][]byte, workChan chan Work, cmdChan chan int, aliveCellsChan chan AliveCells, responseMsgChan chan string, okChan chan bool, paused bool) {

	// Connect to each worker
	var err error
	workerClients := make([]*rpc.Client, numWorkers)
	fmt.Println()
	for i := 0; i < numWorkers; i++ {
		workerClients[i], err = rpc.Dial("tcp", workerIPs[i])
		fmt.Println("Connected to worker: ", workerIPs[i])
		if err != nil {
			panic(err)
		}
	}
	fmt.Println()

	// Initiate each worker with their worker worlds.
	// This has to be done before the loop, because we want to hand the worlds over to each worker in a RPC call before we can
	// loop through each turn and make them calculate the next state.
	topBottomRows := make([]TopBottomRows, numWorkers)
	if turns != 0 {
		if numWorkers != 1 {
			workerHeights := makeWorkerHeights(numWorkers, len(world))
			workerWorlds := buildWorkerWorlds(workerHeights, world)
			for i := range workerWorlds {
				rows := requestStartWorker(*workerClients[i], workerWorlds[i].world, i)
				topBottomRows[i].TopRow = rows.TopRow
				topBottomRows[i].BottomRow = rows.BottomRow
			}
		} else {
			// just start computation with one worker on the original world
			_ = requestStartWorker(*workerClients[0], world, 0)
		}
	}

	turn := 1 // 0th turn was computed when the workers started
	running = true
	for (turn < turns) && running {
		select {
		case cmd := <-cmdChan:
			switch cmd {
			case requestAliveCells:
				// Query workers to send number of alive cells of their part (excl. halo rows) back
				tempWorld := assembleWorkerParts(workerClients, numWorkers)
				aliveCellsChan <- AliveCells{NumAliveCells: numAliveCells(tempWorld), CompletedTurns: turn}
			case requestPgm:
				// Query workers to send back their part back without halo rows
				workerPGMResults := map[int][][]byte{}
				for i := range workerClients {
					result := requestWorkerPGM(*workerClients[i])
					workerPGMResults[result.workerID] = result.world
				}

				// Put parts together
				pgmWorld := makeWorld(0, 0)
				for i := 0; i < numWorkers; i++ {
					part := workerPGMResults[i]
					pgmWorld = append(pgmWorld, part...)
				}
				workChan <- Work{World: pgmWorld, Turn: turn}
			case requestPause:
				if paused == false {
					responseMsg := fmt.Sprintf("Pausing on turn %d", turn)
					responseMsgChan <- responseMsg
					paused = true
					for paused {
						select {
						case c := <-cmdChan:
							if c == requestPause {
								responseMsgChan <- "Continuing"
								paused = false
							}
						}
					}
				}
			case requestStop:
				fmt.Println("Stopping computation" + "\n")
				running = false
			case requestStopWorkers:
				for i := 0; i < numWorkers; i++ {
					requestStopWorker(*workerClients[i])
				}
				fmt.Println("Stopping computation" + "\n")
				okChan <- true
				time.Sleep(2 * time.Second)
				os.Exit(0)
			}
		default:
		}

		// Calculate the next state and communicate the halo rows in between the workers
		tempTopBottomRows := make([]TopBottomRows, numWorkers)
		for i := 0; i < numWorkers; i++ {
			if numWorkers != 2 {
				newTopRow := topBottomRows[(i+numWorkers-1)%numWorkers].BottomRow
				newBottomRow := topBottomRows[(i+1)%numWorkers].TopRow
				tempTopBottomRows[i] = requestNextState(*workerClients[i], TopBottomRows{TopRow: newTopRow, BottomRow: newBottomRow})
			} else if numWorkers == 1 {
				_ = requestNextState(*workerClients[0], TopBottomRows{TopRow: nil, BottomRow: nil})
			} else {
				newTopRow := topBottomRows[(i+1)%numWorkers].BottomRow
				newBottomRow := topBottomRows[(i+1)%numWorkers].TopRow
				tempTopBottomRows[i] = requestNextState(*workerClients[i], TopBottomRows{TopRow: newTopRow, BottomRow: newBottomRow})
			}
		}

		// Update the top and bottom rows for each of the worker worlds after the next state has been calculated for all of them
		for i := range topBottomRows {
			topBottomRows[i] = tempTopBottomRows[i]
		}

		if turn%10 == 0 && turn != 0 {
			fmt.Println("Turn ", turn, " computed")
		}
		turn++
	}

	var newWorld [][]byte
	if turns != 0 { // this is for the testing framework, if no turns have to be computed we don't want to request the workers for their results
		newWorld = assembleWorkerParts(workerClients, numWorkers)
	}

	if running == true { // only send back if the engine has been running and hasn't been stopped by the controller
		fmt.Println("Sending world back" + "\n")
		if turns != 0 {
			workChan <- Work{World: newWorld, Turn: turn}
			running = false
		} else {
			// This is for the testing framework, since the first step is calculated as a way of initialising the workers we don't want to send back a world
			// that which the next state has been calculated, if the number of turns specified by the testing framework is 0. So send back the old world
			workChan <- Work{World: world, Turn: 0}
			running = false
		}
	}

	for i := 0; i < numWorkers; i++ {
		err := workerClients[i].Close()
		if err != nil {
			panic(err)
		}
	}
}

// Gets the results back from the work channel
func getResults(workChan chan Work) Work {
	result := <-workChan
	return Work{World: result.World, Turn: result.Turn}
}

// Gets the number of alive cells and number of completed turns from the alive cells channel
func getAliveCells(aliveCellsChan chan AliveCells, cmdChan chan int) AliveCells {
	cmdChan <- requestAliveCells
	aliveCells := <-aliveCellsChan
	return AliveCells{NumAliveCells: aliveCells.NumAliveCells, CompletedTurns: aliveCells.CompletedTurns}
}

// Gets the board state
func getPGM(workChan chan Work, cmdChan chan int) Work {
	cmdChan <- requestPgm
	work := <-workChan
	return Work{work.World, work.Turn}
}

// Sends pause command to current process
func pause(cmdChan chan int, responseMsgChan chan string) string {
	cmdChan <- requestPause
	response := <-responseMsgChan
	return response
}

// Commands the engine to stop processing the game
func stop(cmdChan chan int) string {
	if running == true {
		cmdChan <- requestStop
		return "Stopping engine"
	}
	return "Engine is not running"
}

// String to send back to controller when it's been connected to the engine
func reconnect() string {
	return "Controller reconnected to engine"
}

// Engine : used to run functions that respond to requests made by the controller.
// 			Can communicate the work that's being done using a channel
type Engine struct {
	workChan        chan Work
	aliveCellsChan  chan AliveCells
	cmdChan         chan int
	responseMsgChan chan string
	okChan          chan bool
}

// GameOfLife : runs the game of life after getting a request from the controller
func (e *Engine) GameOfLife(req stubs.RequestStart, res *stubs.ResponseStart) (err error) {
	if req.World == nil {
		err = errors.New("a world must be specified")
		res.Message = "invalid world"
		return
	}
	fmt.Println("Starting game of life")
	go gameOfLife(req.NumWorkers, req.Turns, req.World, e.workChan, e.cmdChan, e.aliveCellsChan, e.responseMsgChan, e.okChan, false)
	res.Message = "received world"
	return
}

// GetResults : gets the result after all turns have been computed
func (e *Engine) GetResults(req stubs.RequestResult, res *stubs.ResponseResult) (err error) {
	result := getResults(e.workChan)
	res.World = result.World
	res.Turn = result.Turn
	return
}

// AliveCells : gets the number of alive cells when requested by the controller
func (e *Engine) AliveCells(req stubs.RequestAliveCells, res *stubs.ResponseAliveCells) (err error) {
	aliveCells := getAliveCells(e.aliveCellsChan, e.cmdChan)
	res.NumAliveCells = aliveCells.NumAliveCells
	res.CompletedTurns = aliveCells.CompletedTurns
	return
}

// GetPGM : gets the board state so it can be sent to the controller to be saved as a PGM image
func (e *Engine) GetPGM(req stubs.RequestPGM, res *stubs.ResponsePGM) (err error) {
	boardState := getPGM(e.workChan, e.cmdChan)
	res.World = boardState.World
	res.Turn = boardState.Turn
	return
}

// Pause : pauses the computation
func (e *Engine) Pause(req stubs.RequestPause, res *stubs.ResponsePause) (err error) {
	response := pause(e.cmdChan, e.responseMsgChan)
	res.Message = response
	return
}

// Stop : stops the computation
func (e *Engine) Stop(req stubs.RequestStop, res *stubs.ResponseStop) (err error) {
	res.Message = stop(e.cmdChan)
	return
}

// Status : checks if engine is already running
func (e *Engine) Status(req stubs.RequestStatus, res *stubs.ResponseStatus) (err error) {
	status := running
	res.Running = status
	return
}

// Reconnect : reconnects a controller to the engine while it's processing work
func (e *Engine) Reconnect(req stubs.RequestReconnect, res *stubs.ResponseReconnect) (err error) {
	res.Message = reconnect()
	return
}

// StopWorkers : commands the engine to send out requests to workers to be stopped
func (e *Engine) StopWorkers(req stubs.RequestStopWorkers, res *stubs.ResponseStopWorkers) (err error) {
	e.cmdChan <- requestStopWorkers
	response := <-e.okChan
	res.OK = response
	return
}

func main() {
	workChan := make(chan Work)
	aliveCellsChan := make(chan AliveCells)
	cmdChan := make(chan int)
	responseMsgChan := make(chan string)
	okChan := make(chan bool)
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&Engine{
		workChan:        workChan,
		aliveCellsChan:  aliveCellsChan,
		cmdChan:         cmdChan,
		responseMsgChan: responseMsgChan,
		okChan:          okChan,
	})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
