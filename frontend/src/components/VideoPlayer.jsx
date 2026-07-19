import React from 'react';
import { FiPlayCircle } from "react-icons/fi";

export default function VideoPlayer({ jobs, playingJob, onPlayVideo }) {
  const videoJobs = jobs.filter((job) => job.isVideo && job.state !== 'failed').reverse();

  return (
    <section className="flex-1 flex flex-col lg:flex-row min-h-0 overflow-hidden font-mono text-[0.85rem] m-6 border border-[var(--line)] rounded bg-[var(--panel)]">
      {/* Video Playlist Sidebar (Left sub-pane) */}
      <div className="w-full lg:w-[260px] border-r-0 lg:border-r border-[var(--line)] bg-[#040708]/50 flex flex-col min-h-[160px] lg:min-h-0 overflow-hidden">
        <div className="p-4 border-b border-[var(--line)] shrink-0">
          <span className="text-[0.7rem] text-[var(--muted)] uppercase tracking-wider block mb-1">Playlists</span>
          <h3 className="text-sm font-bold  text-[var(--acid)] m-0">AVAILABLE STREAMS</h3>
        </div>
        <div className="flex-1 overflow-y-auto min-h-0">
          {videoJobs.length === 0 ? (
            <p className="p-4 text-[var(--muted)] m-0 text-xs text-center">No video jobs running.</p>
          ) : (
            videoJobs.map((job) => {
              const isActive = playingJob && playingJob.id === job.id;
              const status = job.status || {};
              return (
                <button
                  key={job.id}
                  onClick={() => onPlayVideo(job.id)}
                  className={`w-full text-left p-4 border-b border-[var(--line)] transition-all duration-150 cursor-pointer flex flex-col gap-1.5 min-h-[72px] ${
                    isActive 
                      ? 'bg-[var(--acid)] text-[#03100b] font-bold' 
                      : 'bg-transparent text-[var(--muted)] hover:bg-[#101617]/50 hover:text-[var(--text)]'
                  }`}
                  style={{ border: 'none', borderBottom: '1px solid var(--line)', borderRadius: '0' }}
                >
                  <span className="truncate w-full text-xs font-semibold">
                    {job.videoName || status.torrentName || 'Video Stream'}
                  </span>
                  <span className="flex justify-between w-full text-[0.7rem] opacity-80">
                    <span className="uppercase">{job.state}</span>
                    <span>{(status.percent || 0).toFixed(2)}%</span>
                  </span>
                </button>
              );
            })
          )}
        </div>
      </div>

      {/* Main Video Viewport (Right sub-pane) */}
      <div className="flex-1 flex flex-col min-h-0 overflow-y-auto bg-[#040708]/80 p-6 gap-6">
        {!playingJob ? (
          <div className="flex-1 flex flex-col justify-center items-center text-center p-8">
            <h2 className="text-base  text-[var(--acid)] m-0 font-bold mb-4">Video Player</h2>
            <p className="text-[var(--muted)] m-0 max-w-sm leading-relaxed">
              No active stream selected. Select a running or completed video download from the playlist on the left.
            </p>
          </div>
        ) : (
          <>
            <div className="flex justify-between items-center gap-3.5 border-b border-[var(--line)] pb-3 shrink-0">
              <h2 className="text-base flex flex-row items-center gap-2 text-[var(--acid)] m-0 font-bold truncate max-w-[80%] uppercase">
                <FiPlayCircle className="shrink-0 text-lg" />
                <span className="truncate">
                  {playingJob.videoName ||
                    playingJob.status?.torrentName ||
                    "Video Stream"}
                </span>
              </h2>
              <span className="text-[0.78rem] uppercase text-[var(--acid)] font-bold">{playingJob.state}</span>
            </div>

            <div className="flex-1 relative bg-black border border-[var(--line)] rounded overflow-hidden shadow-2xl flex items-center justify-center min-h-[220px]">
              <video 
                key={playingJob.id} 
                src={`/api/downloads/stream/${playingJob.id}`} 
                controls 
                autoPlay 
                className="w-full h-full object-contain"
              />
            </div>
          </>
        )}
      </div>
    </section>
  );
}
