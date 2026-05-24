package dht

import (
	"crypto/rand"
	"net"
	"sort"
	"sync"
)

type Contact struct {
	ID   [20]byte
	IP   net.IP
	Port uint16
}

type KBucket struct {
	contacts []Contact
	mu       sync.Mutex
}

const K = 8

func (kb *KBucket) Add(contact Contact) {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	for i, c := range kb.contacts {
		if c.ID == contact.ID {
			kb.contacts = append(kb.contacts[:i], kb.contacts[i+1:]...)
			kb.contacts = append(kb.contacts, contact)
			return
		}
	}

	if len(kb.contacts) < K {
		kb.contacts = append(kb.contacts, contact)
	}
}

func (kb *KBucket) Contacts() []Contact {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	copied := make([]Contact, len(kb.contacts))
	copy(copied, kb.contacts)
	return copied
}

type RoutingTable struct {
	selfID  [20]byte
	buckets [160]*KBucket
}

func NewRoutingTable(selfID [20]byte) *RoutingTable {
	rt := &RoutingTable{selfID: selfID}
	for i := 0; i < 160; i++ {
		rt.buckets[i] = &KBucket{}
	}
	return rt
}

func Distance(id1, id2 [20]byte) [20]byte {
	var dist [20]byte
	for i := 0; i < 20; i++ {
		dist[i] = id1[i] ^ id2[i]
	}
	return dist
}

func BucketIndex(id1, id2 [20]byte) int {
	dist := Distance(id1, id2)
	for i, b := range dist {
		if b != 0 {
			for bit := 0; bit < 8; bit++ {
				if (b & (0x80 >> bit)) != 0 {
					return i*8 + bit
				}
			}
		}
	}
	return 159
}

func (rt *RoutingTable) Add(contact Contact) {
	if contact.ID == rt.selfID {
		return
	}
	idx := BucketIndex(rt.selfID, contact.ID)
	rt.buckets[idx].Add(contact)
}

func (rt *RoutingTable) ClosestNodes(target [20]byte, count int) []Contact {
	var all []Contact
	for _, kb := range rt.buckets {
		all = append(all, kb.Contacts()...)
	}
	sort.Slice(all, func(i, j int) bool {
		distI := Distance(all[i].ID, target)
		distJ := Distance(all[j].ID, target)
		for k := 0; k < 20; k++ {
			if distI[k] != distJ[k] {
				return distI[k] < distJ[k]
			}
		}
		return false
	})
	if len(all) > count {
		return all[:count]
	}
	return all
}

func GenerateRandomID() [20]byte {
	var id [20]byte
	_, _ = rand.Read(id[:])
	return id
}
