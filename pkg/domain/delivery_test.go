package domain

import "testing"

func TestDeliveryTransitions(t *testing.T) {
	states := []State{Pending, Processing, Delivered, Deadletter, State("unknown"), State("")}
	for _, from := range states {
		for _, to := range states {
			t.Run(string(from)+"/"+string(to), func(t *testing.T) {
				want := from == Pending && to == Processing || from == Processing && (to == Pending || to == Delivered || to == Deadletter)
				if got := from.CanTransition(to); got != want {
					t.Fatalf("transition = %v, want %v", got, want)
				}
			})
		}
	}
}
