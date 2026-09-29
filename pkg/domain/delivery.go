package domain

type State string

const (
	Pending    State = "pending"
	Processing State = "processing"
	Delivered  State = "delivered"
	Deadletter State = "deadletter"
)

func (s State) CanTransition(next State) bool {
	switch s {
	case Pending:
		return next == Processing
	case Processing:
		return next == Pending || next == Delivered || next == Deadletter
	default:
		return false
	}
}
