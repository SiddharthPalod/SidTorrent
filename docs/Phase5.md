# SiddTorrent — Phase 5 Evolution

## Initial Phase 5 Plan

* UDP-based DHT node listener
* Kademlia Routing Table (160 K-buckets, K = 8)
* DHT RPC queries:
  * `ping` (alive check)
  * `find_node` (bootstrap & routing table expansion)
  * `get_peers` (iterative search for torrent swarm peers)
* Deliverable: Discover active peers trackerlessly without central HTTP/UDP servers

---

## Problems Encountered

### 1. Bencode Interface Casting & Type Checks
* **Problem**: The local custom `bencode` library returns decoded raw data as `interface{}` consisting of nested `map[string]interface{}`, `[]interface{}`, and raw byte slices (`[]byte`) representing bencoded strings. Interacting with this dynamically parsed tree structure without panic crashes required strict interface type-assertions and validation.
* **Fix**: Added highly defensive type assertions (`m["y"].([]byte)`, `a["id"].([]byte)`) across all incoming UDP packet dispatchers to handle type casting securely.

### 2. UDP Port Collision
* **Problem**: By default, BitTorrent clients try to bind their DHT node to UDP port `6881` (isolated from the TCP listener). If multiple SiddTorrent clients or other active torrent programs run on the same machine, binding to `6881` crashes due to port collisions.
* **Fix**: Programmed a fallback dialer that attempts to bind to port `6881` and, if blocked, immediately falls back to a dynamic port allocation (`:0`), ensuring the DHT node always starts successfully.

### 3. Asynchronous Bootstrap Crawl Latencies
* **Problem**: Bootstrapping the DHT table queries external public routers (`router.bittorrent.com:6881`, etc.) asynchronously over UDP. If the crawler immediately initiates iterative torrent lookups right after sending bootstrap queries, the local routing table is still empty, and no peers are discovered.
* **Fix**: Introduced a dynamic 1.5-second sleep interval during bootstrapping to allow the asynchronous UDP listener thread to process incoming bootstrap `find_node` response packets and populate the routing table.

---

## Resulting Phase 5 Implementation

### XOR Distance Routing Table (`kademlia.go` [NEW])
* **XOR Metric Math (`Distance`)**: Computes the logical bitwise XOR difference between 160-bit node identifiers.
* **Leading Zero Buckets (`BucketIndex`)**: Computes bucket indexes `0-159` based on prefix bitwise similarity, matching Kademlia's tree routing structure.
* **K-Bucket Storage (`KBucket`)**: Thread-safe storage holding up to `K=8` closest contacts, ordered by activity.
* **Closest Node Lookup (`ClosestNodes`)**: Filters and sorts the entire routing table by proximity to a target hash, returning the top K nodes to progress iterative crawls.

### UDP DHT Server & RPC Handlers (`dht.go` [NEW])
* **UDP Socket Listener (`listenLoop`)**: Runs a background listener thread parsing and dispatching raw UDP datagrams.
* **RPC Dispatcher**: Safely parses Mainline DHT queries and returns compliant bencoded responses:
  - `ping` -> Responds with our node ID.
  - `find_node` -> Responds with our node ID and the closest compact contacts we know.
  - `get_peers` -> Responds with the active peer list for the requested `info_hash` if we host them, or closer K-contacts if we do not.

### Iterative Peer Discovery Crawler (`SearchPeers`)
* **Recursive Swarm Crawling**: Initiates lookups by querying our own closest routing table nodes.
* **Dynamic Convergence**: Iteratively contacts returned nodes deeper in the network, gathering closer node contacts or immediate peer endpoints recursively.

### Core Swarm Integration (`main.go` [MODIFY])
* **Parallel Discovery**: Executes tracker queries and DHT node lookup crawls in parallel on torrent load.
* **Uniquely Merged Swarms**: Aggregates and de-duplicates all discovered IPv4/IPv6 peer endpoints into a unified active swarm, preserving robust downloading even if all trackers are offline.

---

## Tests Added (`dht_test.go` [NEW])

### Kademlia Distance & Bucket Routing Test
* Validates XOR distance calculations, checks identical IDs return zero distance, asserts MSB toggle triggers bucket `0` routing index, and verifies sorted closest-node retrieval.

### DHT Node Bootstrap Test
* Verifies local node initialization, random ID generation, UDP port binding, and basic boots routing loop activations.

---

## Final Outcome of Phase 5

Phase 5 introduces complete decentralization to SiddTorrent:
* **Central Trackers Optional**: The client successfully participates in the global Mainline DHT swarm.
* **Fault-Tolerant Swarming**: Discovery fails over automatically between trackers and Kademlia nodes.
* **Trackerless Downloads**: The client can discover swarms and download files entirely trackerlessly!
