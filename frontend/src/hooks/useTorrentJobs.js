import { useReducer, useEffect, useState, useRef } from 'react';
import { DEV_CONFIG } from '../config';
import { 
  checkHealth, 
  getJobs, 
  uploadTorrent as apiUploadTorrent, 
  startDownload as apiStartDownload, 
  cancelDownload as apiCancelDownload 
} from '../api/client';

// Action Types
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

export function useTorrentJobs() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const [outputPath, setOutputPath] = useState("");
  const [fileLabel, setFileLabel] = useState("No torrent uploaded.");
  const [isStarting, setIsStarting] = useState(false);
  const [playingJobId, setPlayingJobId] = useState(null);
  const [activeTab, setActiveTab] = useState('live');
  const fileInputRef = useRef(null);
  const [config, setConfig] = useState(() => {
    const saved = localStorage.getItem("torrent_config");
    return saved ? JSON.parse(saved) : { ...DEV_CONFIG };
  });

  useEffect(() => {
    localStorage.setItem("torrent_config", JSON.stringify(config));
  }, [config]);

  const activeJobIdRef = useRef(state.activeJobId);
  useEffect(() => {
    activeJobIdRef.current = state.activeJobId;
  }, [state.activeJobId]);

  const playVideo = (jobId) => {
    setPlayingJobId(jobId);
    setActiveTab('video');  // auto-switch to video tab when user clicks play
  };

  // Setup periodic refresh
  useEffect(() => {
    const init = async () => {
      try {
        await checkHealth();
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
      const jobs = await getJobs();
      dispatch({ type: ActionTypes.SET_JOBS, payload: jobs });

      const activeJobId = activeJobIdRef.current;
      const active = jobs.find((job) => job.id === activeJobId);
      if (active && active.status) {
        dispatch({ type: ActionTypes.SET_PERCENT, payload: active.status.percent });
        dispatch({ type: ActionTypes.SET_PHASE, payload: active.status.phase });
        dispatch({ type: ActionTypes.SET_MESSAGE, payload: active.status.message });
      } else if (!activeJobId) {
        dispatch({ type: ActionTypes.SET_PERCENT, payload: 0 });
        dispatch({ type: ActionTypes.SET_PHASE, payload: "idle" });
        dispatch({ type: ActionTypes.SET_MESSAGE, payload: "Awaiting torrent coordinates" });
      }

      // Auto-select and switch to first video job that becomes ready
      setPlayingJobId((prev) => {
        const firstVideo = jobs.find((j) => j.isVideo && j.state !== 'failed');
        if (!prev && firstVideo) {
          setActiveTab('video');
          return firstVideo.id;
        }
        return prev;
      });
    } catch (err) {
      console.error("Error refreshing jobs:", err);
    }
  };

  const handleFileChange = () => {
    const file = fileInputRef.current?.files[0];
    setFileLabel(file ? file.name : "No torrent uploaded.");
    dispatch({ type: ActionTypes.SET_UPLOADED_PATH, payload: "" });
    dispatch({ type: ActionTypes.SET_METADATA, payload: null });
    if (file) {
      handleUpload(file);
    }
  };

  const handleUpload = async (selectedFile) => {
    const file = selectedFile || fileInputRef.current?.files[0];
    if (!file) {
      dispatch({ type: ActionTypes.SET_MESSAGE, payload: "Choose a .torrent file first." });
      return;
    }
    dispatch({ type: ActionTypes.SET_MESSAGE, payload: "Uploading torrent file..." });
    try {
      const data = await apiUploadTorrent(file);
      dispatch({ type: ActionTypes.SET_UPLOADED_PATH, payload: data.path });
      dispatch({ type: ActionTypes.SET_METADATA, payload: data });
      if (!outputPath) {
        setOutputPath(data.defaultOut || "");
      }
      dispatch({ type: ActionTypes.SET_MESSAGE, payload: "Torrent metadata loaded successfully." });
      return data.path;
    } catch (err) {
      dispatch({ type: ActionTypes.SET_MESSAGE, payload: err.message });
      throw err;
    }
  };

  const handleStart = async (e) => {
    e.preventDefault();
    // Prevent duplicate starts
    if (isStarting) return;
    setIsStarting(true);
    dispatch({ type: ActionTypes.SET_MESSAGE, payload: 'Starting download...' });

    let currentUploadedPath = state.uploadedTorrentPath;
    if (!currentUploadedPath && fileInputRef.current?.files[0]) {
      try {
        currentUploadedPath = await handleUpload();
      } catch (err) {
        setIsStarting(false);
        return;
      }
    }

    if (!currentUploadedPath) {
      dispatch({ type: ActionTypes.SET_MESSAGE, payload: 'Choose a .torrent file first.' });
      setIsStarting(false);
      return;
    }

    const payload = {
      torrentPath: currentUploadedPath,
      outputPath: outputPath.trim(),
      maxDownloadKB: Number(config.maxDownloadKB || 0),
      maxUploadKB: Number(config.maxUploadKB || 0),
      incomingPort: Number(config.incomingPort || 6881),
      maxActiveConnections: Number(config.maxActiveConnections || 100),
      enableDHT: config.enableDHT !== false,
      enablePEX: config.enablePEX !== false,
      enableStreaming: config.enableStreaming !== false,
      enableChoking: config.enableChoking !== false,
      enableMetrics: config.enableMetrics !== false,
      enableEncryption: config.enableEncryption !== false,
      enableUPnP: config.enableUPnP !== false,
    };

    try {
      const job = await apiStartDownload(payload);
      dispatch({ type: ActionTypes.SET_ACTIVE_JOB_ID, payload: job.id });
      await fetchJobs();
      dispatch({ type: ActionTypes.SET_MESSAGE, payload: 'Download started.' });
    } catch (err) {
      dispatch({ type: ActionTypes.SET_MESSAGE, payload: err.message });
    } finally {
      setIsStarting(false);
    }
  };

  const handleCancel = async () => {
    const activeJobId = state.activeJobId;
    if (!activeJobId) return;
    try {
      await apiCancelDownload(activeJobId);
      dispatch({ type: ActionTypes.SET_ACTIVE_JOB_ID, payload: null });
      await fetchJobs();
    } catch (err) {
      console.error("Failed to cancel job:", err);
    }
  };

  const handleStream = (jobId) => {
    window.open(`/api/downloads/stream/${jobId}`, "_blank");
  };

  const activeJob = state.jobs.find((j) => j.id === state.activeJobId) || state.jobs[0];
  const isCancelDisabled = !activeJob || !["queued", "running"].includes(activeJob.state);

  return {
    isStarting,
    setIsStarting,
    state,
    outputPath,
    setOutputPath,
    fileLabel,
    fileInputRef,
    handleFileChange,
    handleUpload,
    handleStart,
    handleCancel,
    handleStream,
    isCancelDisabled,
    fetchJobs,
    activeTab,
    setActiveTab,
    playingJobId,
    setPlayingJobId,
    playVideo,
    config,
    setConfig,
  };
}
