package domain

// Aligner is an interface which inject alignment messages to enforce the conversation align to user's original prompt.
// It returns a message so that conversation aligns to user's original prompt.
// It also enforces Todos be completed.
// Messages aligner injected should be removed when compacting messages.
type Aligner interface {
	InjectMessage(state State, currentStep, maxStep int)
}
