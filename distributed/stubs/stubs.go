package stubs

var GameOfLifeHandler = "Engine.GameOfLife"
var AliveCellsHandler = "Engine.AliveCells"
var ResultsHandler = "Engine.GetResults"

type ResponseStart struct {
	Message string
}

type ResponseAliveCells struct {
	CompletedTurns int
	NumAliveCells  int
}

type ResponseResult struct {
	World [][]byte
	Turn  int
}

type RequestStart struct {
	World [][]byte
	Turns int
}

type RequestResult struct{}

type RequestAliveCells struct{}
