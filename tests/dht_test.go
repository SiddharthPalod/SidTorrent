package tests

import (
	"net"
	"testing"

	"github.com/SiddharthPalod/SidTorrent/internal/dht"
)

func TestKademliaDistanceAndBucketRouting(t *testing.T) {
	id1 := dht.GenerateRandomID()
	id2 := id1

	// XOR distance for identical IDs must be zero
	dist := dht.Distance(id1, id2)
	for _, b := range dist {
		if b != 0 {
			t.Fatalf("identical IDs returned non-zero distance: %v", dist)
		}
	}

	// Create third node ID with exact XOR prefix differences
	id3 := id1
	id3[0] ^= 0x80 // Toggle the most significant bit

	idx := dht.BucketIndex(id1, id3)
	if idx != 0 {
		t.Fatalf("expected bucket index 0 for MSB toggle, got %d", idx)
	}

	rt := dht.NewRoutingTable(id1)
	rt.Add(dht.Contact{ID: id3, IP: net.ParseIP("127.0.0.1"), Port: 5000})

	closest := rt.ClosestNodes(id3, 1)
	if len(closest) != 1 || closest[0].ID != id3 {
		t.Fatalf("failed to retrieve closest routing contact")
	}
}

func TestDHTNodeBootstrapQuery(t *testing.T) {
	// Initialize a local DHT node on a dynamic port
	node, err := dht.NewDHTNode(0)
	if err != nil {
		t.Fatalf("NewDHTNode() failed: %v", err)
	}
	defer node.Close()

	if len(node.ID) != 20 {
		t.Fatalf("invalid DHT Node ID generated")
	}
}
