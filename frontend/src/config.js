// Developer Configuration
// These values are hidden from the user interface but can be updated here by developers.
export const DEV_CONFIG = {
  // Swarm and Connection Settings
  incomingPort: 6881,
  maxActiveConnections: 100,
  maxDownloadKB: 0, // 0 for unlimited
  maxUploadKB: 0,    // 0 for unlimited

  // Advanced BitTorrent Features (All enabled by default)
  enableDHT: true,
  enablePEX: true,
  enableStreaming: true,
  enableChoking: true,
  enableMetrics: true,
  enableEncryption: true,
  enableUPnP: true,

  // UI Polling Interval
  refreshIntervalMs: 1200
};
