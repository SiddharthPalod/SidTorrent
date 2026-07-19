import React from 'react';
import JobRow from './JobRow';

export default function JobMatrix({ jobs, fetchJobs }) {
  return (
    <section className="m-6 border border-[var(--line)] rounded bg-[var(--panel)] overflow-hidden flex flex-col">
      <div className="flex justify-between items-center gap-3.5 min-h-[56px] px-4 border-b border-[var(--line)] shrink-0">
        <h2 className="text-base  text-[var(--text)] m-0 font-bold">Job Matrix</h2>
        <button 
          className="bg-transparent border border-[var(--acid)] rounded text-[var(--acid)] px-4 py-1.5 font-bold font-mono text-[0.8rem] cursor-pointer hover:brightness-110 transition shrink-0" 
          id="refresh-btn" 
          type="button" 
          onClick={fetchJobs}
        >
          Refresh
        </button>
      </div>
      <div className="flex-1 overflow-y-auto overflow-x-auto min-h-0" id="jobs-table">
        {jobs.length === 0 ? (
          <p className="p-4 text-[var(--muted)] m-0">No active jobs.</p>
        ) : (
          jobs.map((job) => (
            <JobRow 
              key={job.id} 
              job={job} 
            />
          ))
        )}
      </div>
    </section>
  );
}
