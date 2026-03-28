package game

// Engine is the interface all game types implement
type Engine interface {
	ProcessThrow(t Throw) ThrowResult
	Undo() *GameState
	State() *GameState
	IsVisitComplete() bool
	NextPlayer()
	CheckoutHint(score int) string
	GetID() string
}

// NewEngine creates the appropriate engine for the game options
func NewEngine(opts GameOptions) Engine {
	switch opts.GameType {
	case "x01":
		return NewX01Engine(opts)
	case "cricket":
		return NewCricketEngine(opts)
	case "atc":
		return NewATCEngine(opts)
	case "shanghai":
		return NewShanghaiEngine(opts)
	case "highscore":
		return NewHighScoreEngine(opts)
	default:
		return NewX01Engine(opts)
	}
}
