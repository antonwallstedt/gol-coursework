package stubs

var GameOfLifeHandler = "Engine.GameOfLife"
var AliveCellsHandler = "Engine.AliveCells"
var ResultsHandler = "Engine.GetResults"
var PGMHandler = "Engine.GetPGM"
var PauseHandler = "Engine.Pause"
var ContinueHandler = "Engine.Continue"

type ResponseStart struct {
	Message string
}

type ResponseAliveCells struct {
	CompletedTurns int
	NumAliveCells  int
}
type RequestContinue struct {
}
type ResponceContinue struct {
	Message string
}
type ResponseResult struct {
	World [][]byte
	Turn  int
}

type ResponsePGM struct {
	World [][]byte
	Turn  int
}

type ResponsePause struct {
	Message string
}

type RequestStart struct {
	World [][]byte
	Turns int
}

type RequestResult struct{}

type RequestAliveCells struct{}

type RequestPGM struct{}

type RequestPause struct{}
