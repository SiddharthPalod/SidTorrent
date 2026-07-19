import React from 'react';

export default function JobRow({ job }) {
  const status = job.status || {};
  const request = job.request || {};

  return (
    <div className="grid grid-cols-[110px_100px_1fr_110px_90px] gap-3.5 items-center min-w-[700px] p-3.5 border-t border-[var(--line)] first:border-t-0 font-mono text-[0.85rem]">
      <b className="text-[var(--acid)]">{job.state}</b>
      <span className="text-[var(--text)]">{(status.percent || 0).toFixed(2)}%</span>
      <small className="text-[var(--muted)] max-w-[300px] overflow-hidden text-ellipsis whitespace-nowrap">
        {status.torrentName || request.torrentPath || job.id}
      </small>
      <span className="text-[var(--text)]">{status.activePeers || 0}/{status.peerCount || 0} peers</span>
      <small className="text-[var(--muted)]">{status.completed || 0}/{status.totalPieces || 0}</small>
    </div>
  );
}
