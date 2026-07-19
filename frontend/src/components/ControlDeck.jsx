import React from 'react';

export default function ControlDeck({ config, setConfig }) {
  const handleToggle = (key) => {
    setConfig((prev) => ({
      ...prev,
      [key]: !prev[key],
    }));
  };

  const handleValueChange = (key, val) => {
    setConfig((prev) => ({
      ...prev,
      [key]: val,
    }));
  };

  return (
    <section className="flex-1 flex flex-col m-6 border border-[var(--line)] rounded bg-[var(--panel)] overflow-hidden min-h-0">
      {/* Panel Title */}
      <div className="flex justify-between items-center gap-3.5 min-h-[56px] px-4 border-b border-[var(--line)] shrink-0">
        <h2 className="text-base  text-[var(--text)] m-0 font-bold">Control Deck</h2>
        <span className="text-[0.78rem] uppercase text-[var(--acid)] font-bold">Active Configuration</span>
      </div>

      {/* Control Grid Content */}
      <div className="flex-1 p-6 overflow-y-auto flex flex-col gap-6">
        {/* Connection & Limits Block */}
        <div className="border border-[var(--line)] rounded bg-[var(--panel-strong)] p-4 flex flex-col gap-4 flex-1">
          <h3 className="text-xs font-bold  text-[var(--text)] uppercase tracking-wider m-0">Bandwidth & Connection Limits</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="flex flex-col gap-1.5">
              <span className="text-[var(--muted)] text-[0.7rem] uppercase">Max Download (KB/s)</span>
              <input 
                type="number"
                value={config.maxDownloadKB || 0}
                onChange={(e) => handleValueChange("maxDownloadKB", Number(e.target.value))}
                className="w-full bg-[#071010] border border-[#33534e] rounded px-3 py-1.5 text-[var(--text)] outline-none focus:border-[var(--acid)] text-xs"
              />
              <small className="text-[var(--muted)] text-[0.65rem]">0 for unlimited</small>
            </div>
            <div className="flex flex-col gap-1.5">
              <span className="text-[var(--muted)] text-[0.7rem] uppercase">Max Upload (KB/s)</span>
              <input 
                type="number"
                value={config.maxUploadKB || 0}
                onChange={(e) => handleValueChange("maxUploadKB", Number(e.target.value))}
                className="w-full bg-[#071010] border border-[#33534e] rounded px-3 py-1.5 text-[var(--text)] outline-none focus:border-[var(--acid)] text-xs"
              />
              <small className="text-[var(--muted)] text-[0.65rem]">0 for unlimited</small>
            </div>
            <div className="flex flex-col gap-1.5">
              <span className="text-[var(--muted)] text-[0.7rem] uppercase">Incoming Port</span>
              <input 
                type="number"
                value={config.incomingPort || 6881}
                onChange={(e) => handleValueChange("incomingPort", Number(e.target.value))}
                className="w-full bg-[#071010] border border-[#33534e] rounded px-3 py-1.5 text-[var(--text)] outline-none focus:border-[var(--acid)] text-xs"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <span className="text-[var(--muted)] text-[0.7rem] uppercase">Max Connections</span>
              <input 
                type="number"
                value={config.maxActiveConnections || 100}
                onChange={(e) => handleValueChange("maxActiveConnections", Number(e.target.value))}
                className="w-full bg-[#071010] border border-[#33534e] rounded px-3 py-1.5 text-[var(--text)] outline-none focus:border-[var(--acid)] text-xs"
              />
            </div>
          </div>
        </div>

        {/* Feature Switches Block */}
        <div className="border border-[var(--line)] rounded bg-[var(--panel-strong)] p-4 flex flex-col gap-4">
          <h3 className="text-xs font-bold  text-[var(--text)] uppercase tracking-wider m-0">Advanced Client Settings</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            
            {/* DHT */}
            <label className="flex items-center justify-between p-3 border border-[var(--line)] rounded bg-[#071010]/30 cursor-pointer hover:bg-[#071010]/70 transition">
              <div className="flex flex-col gap-0.5">
                <span className="text-xs font-bold text-[var(--text)]">Enable DHT</span>
                <span className="text-[0.68rem] text-[var(--muted)]">Distributed Hash Table</span>
              </div>
              <input 
                type="checkbox"
                checked={config.enableDHT}
                onChange={() => handleToggle("enableDHT")}
                className="w-4 h-4 accent-[var(--acid)] cursor-pointer"
              />
            </label>

            {/* PEX */}
            <label className="flex items-center justify-between p-3 border border-[var(--line)] rounded bg-[#071010]/30 cursor-pointer hover:bg-[#071010]/70 transition">
              <div className="flex flex-col gap-0.5">
                <span className="text-xs font-bold text-[var(--text)]">Enable PEX</span>
                <span className="text-[0.68rem] text-[var(--muted)]">Peer Exchange Protocol</span>
              </div>
              <input 
                type="checkbox"
                checked={config.enablePEX}
                onChange={() => handleToggle("enablePEX")}
                className="w-4 h-4 accent-[var(--acid)] cursor-pointer"
              />
            </label>

            {/* Streaming */}
            <label className="flex items-center justify-between p-3 border border-[var(--line)] rounded bg-[#071010]/30 cursor-pointer hover:bg-[#071010]/70 transition">
              <div className="flex flex-col gap-0.5">
                <span className="text-xs font-bold text-[var(--text)]">Enable Streaming</span>
                <span className="text-[0.68rem] text-[var(--muted)]">Sequential piece requests</span>
              </div>
              <input 
                type="checkbox"
                checked={config.enableStreaming}
                onChange={() => handleToggle("enableStreaming")}
                className="w-4 h-4 accent-[var(--acid)] cursor-pointer"
              />
            </label>

            {/* Choking */}
            <label className="flex items-center justify-between p-3 border border-[var(--line)] rounded bg-[#071010]/30 cursor-pointer hover:bg-[#071010]/70 transition">
              <div className="flex flex-col gap-0.5">
                <span className="text-xs font-bold text-[var(--text)]">Enable Choking</span>
                <span className="text-[0.68rem] text-[var(--muted)]">Algorithmic choking rates</span>
              </div>
              <input 
                type="checkbox"
                checked={config.enableChoking}
                onChange={() => handleToggle("enableChoking")}
                className="w-4 h-4 accent-[var(--acid)] cursor-pointer"
              />
            </label>

            {/* Metrics */}
            <label className="flex items-center justify-between p-3 border border-[var(--line)] rounded bg-[#071010]/30 cursor-pointer hover:bg-[#071010]/70 transition">
              <div className="flex flex-col gap-0.5">
                <span className="text-xs font-bold text-[var(--text)]">Enable Metrics</span>
                <span className="text-[0.68rem] text-[var(--muted)]">Export live scrape metrics</span>
              </div>
              <input 
                type="checkbox"
                checked={config.enableMetrics}
                onChange={() => handleToggle("enableMetrics")}
                className="w-4 h-4 accent-[var(--acid)] cursor-pointer"
              />
            </label>

            {/* Encryption */}
            <label className="flex items-center justify-between p-3 border border-[var(--line)] rounded bg-[#071010]/30 cursor-pointer hover:bg-[#071010]/70 transition">
              <div className="flex flex-col gap-0.5">
                <span className="text-xs font-bold text-[var(--text)]">Encryption</span>
                <span className="text-[0.68rem] text-[var(--muted)]">Enforce stream encryption</span>
              </div>
              <input 
                type="checkbox"
                checked={config.enableEncryption}
                onChange={() => handleToggle("enableEncryption")}
                className="w-4 h-4 accent-[var(--acid)] cursor-pointer"
              />
            </label>

            {/* UPnP */}
            <label className="flex items-center justify-between p-3 border border-[var(--line)] rounded bg-[#071010]/30 cursor-pointer hover:bg-[#071010]/70 transition col-span-1 md:col-span-2 lg:col-span-1">
              <div className="flex flex-col gap-0.5">
                <span className="text-xs font-bold text-[var(--text)]">UPnP Port Mapping</span>
                <span className="text-[0.68rem] text-[var(--muted)]">Universal Plug and Play mapping</span>
              </div>
              <input 
                type="checkbox"
                checked={config.enableUPnP}
                onChange={() => handleToggle("enableUPnP")}
                className="w-4 h-4 accent-[var(--acid)] cursor-pointer"
              />
            </label>

          </div>
        </div>
      </div>
    </section>
  );
}
