import React, { useReducer, useEffect, useState, useRef } from 'react';
import { DEV_CONFIG } from './config';

// Actions
const ActionTypes = {
  SET_HEALTH: 'SET_HEALTH',
  SET_METADATA: 'SET_METADATA',
  SET_JOBS: 'SET_JOBS',
  SET_ACTIVE_JOB_ID: 'SET_ACTIVE_JOB_ID',
  SET_UPLOADED_PATH: 'SET_UPLOADED_PATH',
  SET_MESSAGE: 'SET_MESSAGE',
  SET_PHASE: 'SET_PHASE',
  SET_PERCENT: 'SET_PERCENT',
};

// Reducer
const reducer = (state, action) => {
  switch (action.type) {
    case ActionTypes.SET_HEALTH:
      return { ...state, healthOnline: action.payload };
    case ActionTypes.SET_METADATA:
      return { ...state, metadata: action.payload };
    case ActionTypes.SET_JOBS:
      return { ...state, jobs: action.payload };
    case ActionTypes.SET_ACTIVE_JOB_ID:
      return { ...state, activeJobId: action.payload };
    case ActionTypes.SET_UPLOADED_PATH:
      return { ...state, uploadedTorrentPath: action.payload };
    case ActionTypes.SET_MESSAGE:
      return { ...state, message: action.payload };
    case ActionTypes.SET_PHASE:
      return { ...state, phase: action.payload };
    case ActionTypes.SET_PERCENT:
      return { ...state, percent: action.payload };
    default:
      return state;
  }
};

const initialState = {
  healthOnline: false,
  metadata: null,
  jobs: [],
  activeJobId: null,
  uploadedTorrentPath: "",
  message: "Awaiting torrent coordinates.",
  phase: "idle",
  percent: 0,
};

// API Helpers
const api = async (path, options = {}) => {
  const response = await fetch(path, options);
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.error || `Request failed: ${response.status}`);
  }
  return payload;
};

const jsonApi = (path, options = {}) =>
  api(path, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });

const formatBytes = (value) => {
  if (!value) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let index = 0;
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024;
    index += 1;
  }
  return `${size.toFixed(index === 0 ? 0 : 2)} ${units[index]}`;
};

// Job Row component with localized Copy state
function JobRow({ job, onStream }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    const streamUrl = `${window.location.origin}/api/downloads/stream/${job.id}`;
    navigator.clipboard.writeText(streamUrl).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    }).catch(err => {
      console.error("Failed to copy URL:", err);
    });
  };

  const status = job.status || {};
  const request = job.request || {};
  const features = [
    request.enableDHT === false ? null : "DHT",
    request.enablePEX === false ? null : "PEX",
    request.enableStreaming === false ? null : "Streaming",
    request.enableChoking === false ? null : "Choking",
    request.enableMetrics === false ? null : "Metrics",
    request.enableEncryption ? "Enc" : null,
    request.enableUPnP ? "UPnP" : null,
  ].filter(Boolean);

  return (
    <div className="job-row">
      <b>{job.state}</b>
      <span>{(status.percent || 0).toFixed(2)}%</span>
      <small style={{ maxWidth: '300px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {status.torrentName || request.torrentPath || job.id}
      </small>
      <span>{status.activePeers || 0}/{status.peerCount || 0} peers</span>
      <small>{status.completed || 0}/{status.totalPieces || 0}</small>
      <div style={{ display: 'flex', gap: '8px', alignItems: 'center', justifyContent: 'flex-end' }}>
        {job.isVideo && (
          <>
            <button 
              className="stream-btn" 
              onClick={() => onStream(job.id)}
            >
              📺 Stream
            </button>
            <button 
              className="copy-vlc-btn" 
              onClick={handleCopy}
              style={{ 
                background: 'transparent', 
                border: copied ? '1px solid var(--acid)' : '1px solid var(--cyan)', 
                color: copied ? 'var(--acid)' : 'var(--cyan)', 
                borderRadius: '4px', 
                padding: '4px 8px', 
                fontSize: '0.75rem', 
                cursor: 'pointer', 
                transition: 'all 0.2s ease',
                height: '28px',
                minHeight: '28px'
              }}
            >
              {copied ? 'Copied!' : '🔗 VLC Link'}
            </button>
          </>
        )}
        <small style={{ maxWidth: '100px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {features.join(", ") || "Simple"}
        </small>
      </div>
    </div>
  );
}

export default function App() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const [outputPath, setOutputPath] = useState("");
  const [fileLabel, setFileLabel] = useState("No torrent uploaded.");
  const fileInputRef = useRef(null);

  // Setup periodic refresh
  useEffect(() => {
    // Initial health check
    const init = async () => {
      try {
        await api("/api/health");
        dispatch({ type: ActionTypes.SET_HEALTH, payload: true });
        await fetchJobs();
      } catch (err) {
        dispatch({ type: ActionTypes.SET_HEALTH, payload: false });
      }
    };
    init();

    const interval = setInterval(fetchJobs, DEV_CONFIG.refreshIntervalMs || 1200);
    return () => clearInterval(interval);
  }, [state.activeJobId]);

  const fetchJobs = async () => {
    try {
      const jobs = await api("/api/downloads");
      dispatch({ type: ActionTypes.SET_JOBS, payload: jobs });
      
      const activeJobId = state.activeJobId;
      const active = jobs.find((job) => job.id === activeJobId) || jobs[0];
      if (active && active.status) {
        dispatch({ type: ActionTypes.SET_PERCENT, payload: active.status.percent });
        dispatch({ type: ActionTypes.SET_PHASE, payload: active.status.phase });
        dispatch({ type: ActionTypes.SET_MESSAGE, payload: active.status.message });
      }
    } catch (err) {
      console.error("Error refreshing jobs:", err);
    }
  };

  const handleFileChange = () => {
    const file = fileInputRef.current?.files[0];
    setFileLabel(file ? file.name : "No torrent uploaded.");
    dispatch({ type: ActionTypes.SET_UPLOADED_PATH, payload: "" });
    dispatch({ type: ActionTypes.SET_METADATA, payload: null });
  };

  const handleUpload = async () => {
    const file = fileInputRef.current?.files[0];
    if (!file) {
      dispatch({ type: ActionTypes.SET_MESSAGE, payload: "Choose a .torrent file first." });
      return;
    }
    dispatch({ type: ActionTypes.SET_MESSAGE, payload: "Uploading torrent file..." });
    try {
      const formData = new FormData();
      formData.append("torrent", file);
      const data = await api("/api/torrents/upload", {
        method: "POST",
        body: formData,
      });
      dispatch({ type: ActionTypes.SET_UPLOADED_PATH, payload: data.path });
      dispatch({ type: ActionTypes.SET_METADATA, payload: data });
      if (!outputPath) {
        setOutputPath(data.defaultOut || "");
      }
      dispatch({ type: ActionTypes.SET_MESSAGE, payload: "Torrent metadata loaded successfully." });
    } catch (err) {
      dispatch({ type: ActionTypes.SET_MESSAGE, payload: err.message });
    }
  };

  const handleStart = async (e) => {
    e.preventDefault();
    let currentUploadedPath = state.uploadedTorrentPath;
    if (!currentUploadedPath && fileInputRef.current?.files[0]) {
      await handleUpload();
      // Need to read the updated path, but since useReducer is async, we do the upload call return
      const file = fileInputRef.current?.files[0];
      const formData = new FormData();
      formData.append("torrent", file);
      const data = await api("/api/torrents/upload", { method: "POST", body: formData });
      currentUploadedPath = data.path;
    }

    if (!currentUploadedPath) return;

    const payload = {
      torrentPath: currentUploadedPath,
      outputPath: outputPath.trim(),
      maxDownloadKB: Number(DEV_CONFIG.maxDownloadKB || 0),
      maxUploadKB: Number(DEV_CONFIG.maxUploadKB || 0),
      incomingPort: Number(DEV_CONFIG.incomingPort || 6881),
      maxActiveConnections: Number(DEV_CONFIG.maxActiveConnections || 100),
      enableDHT: DEV_CONFIG.enableDHT !== false,
      enablePEX: DEV_CONFIG.enablePEX !== false,
      enableStreaming: DEV_CONFIG.enableStreaming !== false,
      enableChoking: DEV_CONFIG.enableChoking !== false,
      enableMetrics: DEV_CONFIG.enableMetrics !== false,
      enableEncryption: DEV_CONFIG.enableEncryption !== false,
      enableUPnP: DEV_CONFIG.enableUPnP !== false,
    };

    try {
      const job = await jsonApi("/api/downloads", {
        method: "POST",
        body: JSON.stringify(payload),
      });
      dispatch({ type: ActionTypes.SET_ACTIVE_JOB_ID, payload: job.id });
      await fetchJobs();
    } catch (err) {
      dispatch({ type: ActionTypes.SET_MESSAGE, payload: err.message });
    }
  };

  const handleCancel = async () => {
    const activeJobId = state.activeJobId;
    if (!activeJobId) return;
    try {
      await jsonApi(`/api/downloads/${activeJobId}`, { method: "DELETE" });
      await fetchJobs();
    } catch (err) {
      console.error("Failed to cancel job:", err);
    }
  };

  const handleStream = (jobId) => {
    window.open(`/api/downloads/stream/${jobId}`, "_blank");
  };

  // Determine cancel button state
  const activeJob = state.jobs.find((j) => j.id === state.activeJobId) || state.jobs[0];
  const isCancelDisabled = !activeJob || !["queued", "running"].includes(activeJob.state);

  const clamped = Math.max(0, Math.min(100, Number(state.percent || 0)));

  return (
    <main className="shell">
      <section className="console">
        <header className="topbar">
          <div>
            <p className="kicker">SiddTorrent</p>
            <h1>Swarm Control</h1>
          </div>
          <div className={`health ${state.healthOnline ? 'online' : 'offline'}`} id="health">
            <span className="pulse"></span>
            <span>{state.healthOnline ? 'api online' : 'api offline'}</span>
          </div>
        </header>

        <form className="launch" id="torrent-form" onSubmit={handleStart}>
          <div className="file-picker">
            <label>
              <span>Torrent upload</span>
              <input 
                ref={fileInputRef}
                id="torrent-file" 
                name="torrentFile" 
                type="file" 
                accept=".torrent,application/x-bittorrent" 
                onChange={handleFileChange}
                required 
              />
            </label>
            <small id="torrent-path">{fileLabel}</small>
          </div>
          <label>
            <span>Output path</span>
            <input 
              id="output-path" 
              name="outputPath" 
              autoComplete="off" 
              placeholder="downloads\\file.iso" 
              value={outputPath}
              onChange={(e) => setOutputPath(e.target.value)}
            />
          </label>
          <div className="actions">
            <button type="button" id="inspect-btn" onClick={handleUpload}>Upload</button>
            <button type="submit">Start</button>
          </div>
        </form>

        <section className="grid">
          <article className="panel meta">
            <div className="panel-head">
              <h2>Torrent Intel</h2>
              <span id="meta-state">{state.metadata ? 'loaded' : 'idle'}</span>
            </div>
            <dl id="metadata">
              <div>
                <dt>Name</dt>
                <dd>{state.metadata ? state.metadata.name : '-'}</dd>
              </div>
              <div>
                <dt>Size</dt>
                <dd>{state.metadata ? formatBytes(state.metadata.length) : '-'}</dd>
              </div>
              <div>
                <dt>Pieces</dt>
                <dd>{state.metadata ? `${state.metadata.pieceCount} x ${formatBytes(state.metadata.pieceLength)}` : '-'}</dd>
              </div>
              <div>
                <dt>Info hash</dt>
                <dd>{state.metadata ? state.metadata.infoHash : '-'}</dd>
              </div>
              <div>
                <dt>Tracker</dt>
                <dd>{state.metadata ? (state.metadata.announce || 'trackerless') : '-'}</dd>
              </div>
            </dl>
          </article>

          <article className="panel status">
            <div className="panel-head">
              <h2>Live Session</h2>
              <button 
                className="ghost" 
                id="cancel-btn" 
                type="button" 
                disabled={isCancelDisabled}
                onClick={handleCancel}
              >
                Cancel
              </button>
            </div>
            <div className="progress-ring">
              <svg viewBox="0 0 120 120" aria-hidden="true">
                <circle cx="60" cy="60" r="52"></circle>
                <circle 
                  id="progress-circle" 
                  cx="60" 
                  cy="60" 
                  r="52" 
                  style={{ strokeDashoffset: 327 - (327 * clamped) / 100 }}
                ></circle>
              </svg>
              <strong id="percent">{clamped.toFixed(2)}%</strong>
            </div>
            <div className="readout">
              <span id="phase">{state.phase}</span>
              <p id="message">{state.message}</p>
            </div>
          </article>
        </section>

        <section className="panel jobs">
          <div className="panel-head">
            <h2>Job Matrix</h2>
            <button className="ghost" id="refresh-btn" type="button" onClick={fetchJobs}>Refresh</button>
          </div>
          <div className="jobs-table" id="jobs-table">
            {state.jobs.length === 0 ? (
              <p className="empty">No active jobs.</p>
            ) : (
              state.jobs.map((job) => (
                <JobRow 
                  key={job.id} 
                  job={job} 
                  onStream={handleStream}
                />
              ))
            )}
          </div>
        </section>
      </section>
    </main>
  );
}
