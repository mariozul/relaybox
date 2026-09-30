package observability

import "testing"

func TestDeliveryStatusLabelsAreBounded(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"delivered", "failed", "deadletter"} {
		if !ValidDeliveryStatus(status) {
			t.Fatalf("rejected %q", status)
		}
	}
	if ValidDeliveryStatus("tenant-a") {
		t.Fatal("accepted unbounded status")
	}
}
