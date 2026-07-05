# SiddTorrent

SiddTorrent is a feature-rich, high-performance BitTorrent engine and web console implemented in Go. It supports multi-peer swarm orchestration, rarest-first piece scheduling, sequential streaming priorities, token-bucket rate limiting, automatic corruption detection and IP blacklisting, extension protocol handshakes, and full linear multi-file output mapping.

---

## 🚀 Primary Interface: Web Console

The recommended way to control SiddTorrent is via the built-in **Web Console**, which acts as the main control surface. It features a modern SPA (Single Page Application) dashboard to upload `.torrent` files, view real-time swarm download percentages, manage active download jobs, and toggle advanced download engine components.

### Start the Web & API Server
```powershell
go run ./cmd/siddtorrent-api -addr 127.0.0.1:8081 -static web
```
Then, open your web browser and navigate to:
👉 **[http://127.0.0.1:8081](http://127.0.0.1:8081)**

---

## 💻 Frontend Development (React + Vite)

The web console is a modern React application powered by Vite, featuring a state management architecture matching Redux.

All advanced settings, connection caps, and toggles (DHT, PEX, Encryption, UPnP, Choking, etc.) are hidden from the user interface to ensure a clean, consumer-focused UX. 

### Custom Developer Configurations
Developers can configure hidden advanced properties (such as ports, connection caps, and toggles) directly in:
👉 **[config.js](file:///d:/New%20folder%20(4)/SidFiles/Projectd/Goofy-Projects/BitTorrent/frontend/src/config.js)**

### Development Mode (with Hot Reloading)
To run the frontend in development mode with Hot Module Replacement (HMR):
1. **Start the Go Backend Server:**
   ```powershell
   go run ./cmd/siddtorrent-api -addr 127.0.0.1:8081 -static web
   ```
2. **Start the Vite Dev Server:**
   ```powershell
   cd frontend
   npm run dev
   ```
3. Open 👉 **[http://localhost:5173](http://localhost:5173)** in your browser. All UI changes will hot-reload instantly, and api calls will be proxied to the Go server on port 8081.

### Build for Production
To bundle the React frontend into static assets and deploy them directly to the Go server's static directory (`web/`):
```powershell
cd frontend
npm run build
```

---

## 🛠 Secondary Interface: Debugging CLI

For direct terminal runs and debug actions, you can use the single-command CLI:
```powershell
# Basic download
go run ./cmd/siddtorrent path/to/file.torrent

# Download to custom location with rate limits
go run ./cmd/siddtorrent -out downloads/ubuntu.iso -max-download 500 path/to/file.torrent
```

### CLI Flags
- `-out`: Custom path for output file (defaults to `downloads/<torrent-name>`).
- `-max-download`: Max download speed cap in KB/s (default `0` for unlimited).
- `-dht`: Toggle DHT peer discovery (default `true`).
- `-pex`: Toggle Peer Exchange updates (default `true`).
- `-streaming`: Prioritize sequential initial pieces for streaming (default `true`).
- `-choking`: Enable peer Tit-for-Tat choking manager (default `true`).
- `-metrics`: Periodically print statistics (default `true`).

---

## 🏗 Key Features & Architecture

- **Linear Multi-File Mapping**: Recreates the exact multi-file hierarchy from metainfo. Reads and writes crossing physical file boundaries are automatically mapped to respective offsets under the hood.
- **Robust Swarm Orchestration**: Features concurrency-safe rarest-first piece pickers with optimized count buckets, and adaptive choking/unchoking for Tit-For-Tat game-theoretic bandwidth exchange.
- **Sequential Streaming Engine**: Prioritizes initial piece sequences sequentially before falling back to rarest-first, allowing users to play/stream media as it downloads.
- **Corrupt Peer Detection**: Dynamically validates checksums for all pieces on receipt, automatically blacklisting corrupting peer IPs to prevent poison swarms.
- **Experimental Swarm Extensions**:
  - **PEX (Peer Exchange)**: Deadline-safe `ut_pex` implementation supporting IPv4 and IPv6 advertisements.
  - **Mainline DHT**: Basic Kademlia routing and peer queries (`get_peers`/`find_node` queries).

---

## 📂 Package Overview

- [`internal/torrent`](file:///d:/New%20folder%20%284%29/SidFiles/Projectd/Goofy-Projects/BitTorrent/internal/torrent): Bencode decoding, metainfo parsing, and multi-file mappings.
- [`internal/storage`](file:///d:/New%20folder%20%284%29/SidFiles/Projectd/Goofy-Projects/BitTorrent/internal/storage): Linear block-level storage across single and multi-file sets.
- [`internal/disk`](file:///d:/New%20folder%20%284%29/SidFiles/Projectd/Goofy-Projects/BitTorrent/internal/disk): Asynchronous write caching, batching, and metrics.
- [`internal/peer`](file:///d:/New%20folder%20%284%29/SidFiles/Projectd/Goofy-Projects/BitTorrent/internal/peer): Connection handshakes, bitfield exchanges, PEX messages, and wire reader/writer.
- [`internal/piece`](file:///d:/New%20folder%20%284%29/SidFiles/Projectd/Goofy-Projects/BitTorrent/internal/piece): Swarm state trackers, Tit-For-Tat choke managers, rarest-first count buckets, and streaming selectors.
- [`internal/dht`](file:///d:/New%20folder%20%284%29/SidFiles/Projectd/Goofy-Projects/BitTorrent/internal/dht): Kademlia routing tables, UDP queries, and node discovery.
- [`internal/api`](file:///d:/New%20folder%20%284%29/SidFiles/Projectd/Goofy-Projects/BitTorrent/internal/api): HTTP controllers, upload endpoints, and stateful job runners.
