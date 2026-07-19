import React, { useRef, useEffect, useState, useCallback } from 'react';
import { FiPlayCircle, FiRefreshCw, FiWifi, FiAlertCircle } from 'react-icons/fi';

/**
 * VideoPlayer — live stream while downloading, fullscreen-safe reloads
 *
 * Key insight:
 *   Changing the React `key` prop forces a DOM remount which exits fullscreen.
 *   Instead we keep ONE stable <video> element and reload it imperatively:
 *     1. Save currentTime
 *     2. Set video.src = new URL with ?t=<rev> cache-buster
 *     3. Call video.load()   ← tells the browser to re-fetch from the server
 *     4. On canplay, seek back to savedTime and call play()
 *
 *   This keeps the same DOM node → fullscreen is NOT disrupted.
 *
 * Auto-reload logic:
 *   Every 2 s we check job.status.percent. When it grows by ≥ 1 % we trigger
 *   a reload so the player picks up newly downloaded pieces automatically.
 */
export default function VideoPlayer({ jobs, playingJob, onPlayVideo }) {
  const videoRef = useRef(null);

  // Revision counter used only to generate a unique cache-buster query param
  const revRef = useRef(0);

  // Buffered fraction 0-1 as seen by the browser
  const [bufferedFraction, setBufferedFraction] = useState(0);

  // overlay: 'idle' | 'waiting' | 'loading' | 'playing' | 'stalled' | 'error'
  const [overlayState, setOverlayState] = useState('idle');
  const [overlayMsg, setOverlayMsg] = useState('');

  // Track the last percent at which we auto-reloaded
  const lastReloadPctRef = useRef(-1);

  // Save playback position across src swaps
  const savedTimeRef = useRef(0);

  // Keep latest playingJob accessible inside intervals/callbacks
  const playingJobRef = useRef(playingJob);
  useEffect(() => { playingJobRef.current = playingJob; }, [playingJob]);

  const videoJobs = jobs.filter((j) => j.isVideo && j.state !== 'failed').reverse();

  // ── Core reload function (keeps same DOM node → fullscreen safe) ─────────
  const reloadStream = useCallback((seekTo) => {
    const vid = videoRef.current;
    const job = playingJobRef.current;
    if (!vid || !job) return;

    revRef.current += 1;
    const newSrc = `/api/downloads/stream/${job.id}?t=${revRef.current}`;

    // Save position before reload (unless caller already provided one)
    savedTimeRef.current = seekTo ?? (vid.currentTime > 0.5 ? vid.currentTime : 0);

    setOverlayState('loading');
    setOverlayMsg('Reloading stream…');

    vid.src = newSrc;   // update src in-place — same DOM node, no remount
    vid.load();         // re-fetch from server
  }, []);

  // ── Auto-reload while torrent is downloading ─────────────────────────────
  useEffect(() => {
    if (!playingJob || playingJob.state !== 'running') return;

    const iv = setInterval(() => {
      const job = playingJobRef.current;
      if (!job || job.state !== 'running') return;
      const pct = job.status?.percent ?? 0;
      if (pct - lastReloadPctRef.current >= 1) {
        lastReloadPctRef.current = pct;
        reloadStream();
      }
    }, 2000);

    return () => clearInterval(iv);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [playingJob?.id, playingJob?.state]);

  // ── Attach/wire video event handlers ─────────────────────────────────────
  useEffect(() => {
    const vid = videoRef.current;
    if (!vid) return;

    const onCanPlay = () => {
      setOverlayState('playing');
      if (savedTimeRef.current > 0.5) {
        vid.currentTime = savedTimeRef.current;
        savedTimeRef.current = 0;
      }
      vid.play().catch(() => {});
    };

    const onPlay = () => setOverlayState('playing');

    const onWaiting = () => {
      setOverlayState('stalled');
      setOverlayMsg('Waiting for pieces to arrive…');
    };

    const onError = () => {
      const code = vid?.error?.code;
      if (
        code === MediaError.MEDIA_ERR_NETWORK ||
        code === MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED
      ) {
        setOverlayState('waiting');
        setOverlayMsg('Stream not ready yet — pieces still arriving…');
      } else {
        setOverlayState('error');
        setOverlayMsg(vid?.error?.message || 'Playback error. Try reloading.');
      }
    };

    const onProgress = () => {
      if (!vid.duration || !vid.buffered.length) { setBufferedFraction(0); return; }
      setBufferedFraction(vid.buffered.end(vid.buffered.length - 1) / vid.duration);
    };

    vid.addEventListener('canplay', onCanPlay);
    vid.addEventListener('play', onPlay);
    vid.addEventListener('waiting', onWaiting);
    vid.addEventListener('error', onError);
    vid.addEventListener('progress', onProgress);
    vid.addEventListener('timeupdate', onProgress);

    return () => {
      vid.removeEventListener('canplay', onCanPlay);
      vid.removeEventListener('play', onPlay);
      vid.removeEventListener('waiting', onWaiting);
      vid.removeEventListener('error', onError);
      vid.removeEventListener('progress', onProgress);
      vid.removeEventListener('timeupdate', onProgress);
    };
  // Mount-time only — the stable videoRef never changes
  }, []);

  // ── Load a new job when the selection changes ─────────────────────────────
  useEffect(() => {
    const vid = videoRef.current;
    if (!playingJob) {
      setOverlayState('idle');
      if (vid) { vid.pause(); vid.src = ''; }
      return;
    }

    lastReloadPctRef.current = -1;
    savedTimeRef.current = 0;
    setBufferedFraction(0);
    revRef.current += 1;

    setOverlayState('loading');
    setOverlayMsg('Connecting to stream…');

    if (vid) {
      vid.src = `/api/downloads/stream/${playingJob.id}?t=${revRef.current}`;
      vid.load();
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [playingJob?.id]);

  // ── Manual reload button ──────────────────────────────────────────────────
  const handleManualReload = () => reloadStream();

  // ── Derived values ────────────────────────────────────────────────────────
  const downloadPct = playingJob?.status?.percent ?? 0;
  const showOverlay = overlayState !== 'playing';

  const overlayConfig = {
    idle:    { color: 'var(--muted)',  icon: null },
    waiting: { color: 'var(--amber)',  icon: <FiWifi className="text-3xl animate-pulse" /> },
    loading: { color: 'var(--acid)',   icon: <FiRefreshCw className="text-3xl animate-spin" /> },
    stalled: { color: 'var(--amber)',  icon: <FiWifi className="text-3xl animate-pulse" /> },
    error:   { color: '#ef4444',       icon: <FiAlertCircle className="text-3xl" /> },
    playing: { color: 'transparent',  icon: null },
  };
  const ov = overlayConfig[overlayState] ?? overlayConfig.idle;

  return (
    <section className="flex-1 flex flex-col lg:flex-row min-h-0 overflow-hidden font-mono text-[0.85rem] m-6 border border-[var(--line)] rounded bg-[var(--panel)]">

      {/* ── Left: Playlist sidebar ──────────────────────────────────────────── */}
      <div className="w-full lg:w-[260px] border-r-0 lg:border-r border-[var(--line)] bg-[#040708]/50 flex flex-col min-h-[160px] lg:min-h-0 overflow-hidden">
        <div className="p-4 border-b border-[var(--line)] shrink-0">
          <span className="text-[0.7rem] text-[var(--muted)] uppercase tracking-wider block mb-1">
            Playlists
          </span>
          <h3 className="text-sm font-bold text-[var(--acid)] m-0">AVAILABLE STREAMS</h3>
        </div>

        <div className="flex-1 overflow-y-auto min-h-0">
          {videoJobs.length === 0 ? (
            <p className="p-4 text-[var(--muted)] m-0 text-xs text-center">
              No video jobs running.
            </p>
          ) : (
            videoJobs.map((job) => {
              const isActive = playingJob?.id === job.id;
              const pct = (job.status?.percent ?? 0).toFixed(1);
              return (
                <button
                  key={job.id}
                  id={`stream-job-${job.id}`}
                  onClick={() => onPlayVideo(job.id)}
                  className={`w-full text-left p-4 transition-all duration-150 cursor-pointer flex flex-col gap-1.5 min-h-[72px] ${
                    isActive
                      ? 'bg-[var(--acid)] text-[#03100b] font-bold'
                      : 'bg-transparent text-[var(--muted)] hover:bg-[#101617]/50 hover:text-[var(--text)]'
                  }`}
                  style={{
                    border: 'none',
                    borderBottom: '1px solid var(--line)',
                    borderRadius: 0,
                  }}
                >
                  <span className="truncate w-full text-xs font-semibold">
                    {job.videoName || job.status?.torrentName || 'Video Stream'}
                  </span>
                  <span className="flex justify-between w-full text-[0.7rem] opacity-80">
                    <span className="uppercase">{job.state}</span>
                    <span>{pct}%</span>
                  </span>
                  {job.state === 'running' && (
                    <div
                      className="w-full h-1 rounded overflow-hidden mt-0.5"
                      style={{ background: isActive ? 'rgba(3,16,11,0.3)' : '#101617' }}
                    >
                      <div
                        className="h-full transition-[width] duration-500"
                        style={{
                          width: `${pct}%`,
                          background: isActive ? '#03100b' : 'var(--acid)',
                          opacity: isActive ? 0.7 : 1,
                        }}
                      />
                    </div>
                  )}
                </button>
              );
            })
          )}
        </div>
      </div>

      {/* ── Right: Video viewport ───────────────────────────────────────────── */}
      <div className="flex-1 flex flex-col min-h-0 overflow-y-auto bg-[#040708]/80 p-6 gap-4">
        {!playingJob ? (
          <div className="flex-1 flex flex-col justify-center items-center text-center p-8">
            <h2 className="text-base text-[var(--acid)] m-0 font-bold mb-4">Video Player</h2>
            <p className="text-[var(--muted)] m-0 max-w-sm leading-relaxed">
              No active stream selected. Select a running or completed video
              download from the playlist on the left.
            </p>
          </div>
        ) : (
          <>
            {/* Title + controls row */}
            <div className="flex justify-between items-center gap-3.5 border-b border-[var(--line)] pb-3 shrink-0">
              <h2 className="text-base flex flex-row items-center gap-2 text-[var(--acid)] m-0 font-bold truncate max-w-[65%] uppercase">
                <FiPlayCircle className="shrink-0 text-lg" />
                <span className="truncate">
                  {playingJob.videoName || playingJob.status?.torrentName || 'Video Stream'}
                </span>
              </h2>
              <div className="flex items-center gap-3 shrink-0">
                <span className="text-[0.78rem] uppercase text-[var(--acid)] font-bold">
                  {playingJob.state}
                </span>
                <button
                  id="reload-stream-btn"
                  title="Pull latest downloaded data into the player without leaving fullscreen"
                  onClick={handleManualReload}
                  className="flex items-center gap-1.5 bg-transparent border border-[var(--line)] text-[var(--muted)] hover:text-[var(--acid)] hover:border-[var(--acid)] rounded px-2.5 py-1 text-[0.72rem] transition-all duration-150 cursor-pointer"
                >
                  <FiRefreshCw className="text-xs" />
                  <span>Reload</span>
                </button>
              </div>
            </div>

            {/* Video + overlay wrapper */}
            <div
              id="video-viewport"
              className="flex-1 relative bg-black border border-[var(--line)] rounded overflow-hidden shadow-2xl flex items-center justify-center min-h-[220px]"
            >
              {/*
                ONE stable <video> node — never remounted.
                We update src imperatively so fullscreen is never interrupted.
              */}
              <video
                ref={videoRef}
                controls
                playsInline
                className="w-full h-full object-contain"
                style={{ display: showOverlay ? 'none' : 'block' }}
              />

              {/* State overlay */}
              {showOverlay && (
                <div
                  className="absolute inset-0 flex flex-col items-center justify-center gap-4 p-8 text-center"
                  style={{ color: ov.color }}
                >
                  {ov.icon && <div style={{ color: ov.color }}>{ov.icon}</div>}
                  {overlayMsg && (
                    <p
                      className="text-xs font-mono m-0 max-w-xs leading-relaxed"
                      style={{ color: ov.color }}
                    >
                      {overlayMsg}
                    </p>
                  )}
                  {(overlayState === 'error' || overlayState === 'stalled') && (
                    <button
                      onClick={handleManualReload}
                      className="mt-1 border border-[var(--acid)] text-[var(--acid)] rounded px-4 py-1.5 text-xs font-mono cursor-pointer hover:brightness-125 transition bg-transparent"
                    >
                      Retry
                    </button>
                  )}
                </div>
              )}
            </div>

            {/* ── Progress bars ─────────────────────────────────────────────── */}
            <div className="shrink-0 flex flex-col gap-1.5">
              <div className="flex justify-between text-[0.68rem] font-mono text-[var(--muted)]">
                <span className="uppercase tracking-wide">Download Progress</span>
                <span className="text-[var(--acid)] font-bold">{downloadPct.toFixed(2)}%</span>
              </div>

              <div className="w-full h-2.5 bg-[#101617] border border-[var(--line)] rounded overflow-hidden relative">
                {/* Ghost: browser buffered region */}
                <div
                  className="absolute inset-y-0 left-0 transition-[width] duration-700"
                  style={{
                    width: `${Math.round(bufferedFraction * 100)}%`,
                    background: 'rgba(0,255,136,0.15)',
                  }}
                />
                {/* Solid: download progress */}
                <div
                  className="absolute inset-y-0 left-0 transition-[width] duration-300 shadow-[0_0_6px_var(--acid)]"
                  style={{ width: `${downloadPct}%`, background: 'var(--acid)' }}
                />
              </div>

              <div className="flex justify-between text-[0.62rem] font-mono text-[var(--muted)]">
                <span className="flex items-center gap-1.5">
                  <span
                    style={{
                      display: 'inline-block', width: 8, height: 8,
                      background: 'rgba(0,255,136,0.35)', borderRadius: 2,
                    }}
                  />
                  Buffered {Math.round(bufferedFraction * 100)}%
                </span>
                <span>
                  {playingJob.state === 'running'
                    ? 'Auto-reloads every ~1% · fullscreen safe'
                    : playingJob.state === 'complete'
                    ? 'Download complete'
                    : playingJob.state}
                </span>
              </div>
            </div>
          </>
        )}
      </div>
    </section>
  );
}
