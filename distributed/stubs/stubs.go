package stubs

var GameOfLifeHandler = "Engine.GameOfLife"
var AliveCellsHandler = "Engine.AliveCells"

type Response struct {
	World [][]byte
	Turn  int
}

type ResponseAliveCells struct {
	CompletedTurns int
	NumAliveCells  int
}

type Request struct {
	World [][]byte
	Turns int
}

type RequestAliveCells struct{}
