import React from 'react';
import { useTorrentJobs } from './hooks/useTorrentJobs';
import table from './assets/table.png';
import monitor from './assets/monitor.png';
import Topbar from './components/Topbar';
import LaunchForm from './components/LaunchForm';
import TorrentIntel from './components/TorrentIntel';
import LiveSession from './components/LiveSession';
import JobMatrix from './components/JobMatrix';
import Sidebar from './components/Sidebar';
import VideoPlayer from './components/VideoPlayer';
import ControlDeck from './components/ControlDeck';

export default function App() {
  const {
    isStarting,
    state,
    outputPath,
    setOutputPath,
    fileLabel,
    fileInputRef,
    handleFileChange,
    handleUpload,
    handleStart,
    handleCancel,
    isCancelDisabled,
    fetchJobs,
    activeTab,
    setActiveTab,
    playingJobId,
    playVideo,
    config,
    setConfig,
  } = useTorrentJobs();

  const hasVideo = state.jobs.some((job) => job.isVideo);
  const playingJob = state.jobs.find((job) => job.id === playingJobId);

  const activeJob = state.jobs.find((j) => j.id === state.activeJobId) || state.jobs[0];
  const isVideoActive = activeJob ? activeJob.isVideo : false;

  return (
    <main className="shell">
      <Topbar healthOnline={state.healthOnline} />


      <div className='relative w-full'>

        {/* Monitor frame */}
        <img
          src={monitor}
          className="mt-4"
          alt=""
        />

        {/* Console panel sits inside the monitor bezel */}
        <section className="absolute inset-5 top-[4%] console flex flex-col z-10">
          {/* Torrent Upload bar – always visible below topbar */}
          <LaunchForm
            fileInputRef={fileInputRef}
            fileLabel={fileLabel}
            handleFileChange={handleFileChange}
            handleUpload={handleUpload}
            outputPath={outputPath}
            setOutputPath={setOutputPath}
            handleStart={handleStart}
            isStarting={isStarting}
          />
          <div className="flex flex-col lg:flex-row flex-1 min-h-0 overflow-hidden">
            <Sidebar
              activeTab={activeTab}
              setActiveTab={setActiveTab}
              hasVideo={hasVideo}
            />

            <div className="flex-1 flex flex-col bg-[#040708]/30 overflow-y-auto min-h-0">
              {activeTab === 'deck' && (
                <ControlDeck
                  config={config}
                  setConfig={setConfig}
                />
              )}

              {activeTab === 'live' && (
                <article className="m-6 border border-[var(--line)] rounded bg-[var(--panel)] overflow-hidden">
                  <LiveSession
                    percent={state.percent}
                    phase={state.phase}
                    message={state.message}
                    isCancelDisabled={isCancelDisabled}
                    handleCancel={handleCancel}
                  />
                </article>
              )}

              {activeTab === 'intel' && (
                <div className="p-6">
                  <TorrentIntel metadata={state.metadata} />
                </div>
              )}

              {activeTab === 'matrix' && (
                <JobMatrix
                  jobs={state.jobs}
                  fetchJobs={fetchJobs}
                />
              )}

              {activeTab === 'video' && (
                <VideoPlayer
                  jobs={state.jobs}
                  playingJob={playingJob}
                  onPlayVideo={playVideo}
                />
              )}
            </div>
          </div>
        </section>

      </div>

      <div className='h-0 overflow-visible -z-10'>
        <img src={table} className='w-screen -translate-y-40 scale-125 origin-center' alt="" />
      </div>
    </main>
  );
}
