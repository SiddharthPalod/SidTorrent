import React from 'react';
import { formatBytes } from '../api/client';

export default function TorrentIntel({ metadata }) {
  return (
    <article className="flex-1 md:flex-[1.3] border border-[var(--line)] rounded bg-[var(--panel)]">
      <div className="flex justify-between items-center gap-3.5 min-h-[56px] px-4 border-b border-[var(--line)]">
        <h2 className="text-base text-[var(--text)] m-0 font-bold">Torrent Intel</h2>
        <span className="text-[0.78rem] uppercase text-[var(--amber)]">{metadata ? 'loaded' : 'idle'}</span>
      </div>
      <dl className="grid gap-[1px] bg-[var(--line)] m-0">
        <div className="grid grid-cols-[120px_1fr] gap-3.5 min-h-[48px] p-3 bg-[var(--panel-strong)] items-center">
          <dt className="text-[var(--muted)] m-0">Name</dt>
          <dd className="m-0 break-all text-[var(--text)]">{metadata ? metadata.name : '-'}</dd>
        </div>
        <div className="grid grid-cols-[120px_1fr] gap-3.5 min-h-[48px] p-3 bg-[var(--panel-strong)] items-center">
          <dt className="text-[var(--muted)] m-0">Size</dt>
          <dd className="m-0 break-all text-[var(--text)]">{metadata ? formatBytes(metadata.length) : '-'}</dd>
        </div>
        <div className="grid grid-cols-[120px_1fr] gap-3.5 min-h-[48px] p-3 bg-[var(--panel-strong)] items-center">
          <dt className="text-[var(--muted)] m-0">Pieces</dt>
          <dd className="m-0 break-all text-[var(--text)]">
            {metadata ? `${metadata.pieceCount} x ${formatBytes(metadata.pieceLength)}` : '-'}
          </dd>
        </div>
        <div className="grid grid-cols-[120px_1fr] gap-3.5 min-h-[48px] p-3 bg-[var(--panel-strong)] items-center">
          <dt className="text-[var(--muted)] m-0">Info hash</dt>
          <dd className="m-0 break-all font-mono text-[var(--text)]">{metadata ? metadata.infoHash : '-'}</dd>
        </div>
        <div className="grid grid-cols-[120px_1fr] gap-3.5 min-h-[48px] p-3 bg-[var(--panel-strong)] items-center">
          <dt className="text-[var(--muted)] m-0">Tracker</dt>
          <dd className="m-0 break-all text-[var(--text)]">{metadata ? (metadata.announce || 'trackerless') : '-'}</dd>
        </div>
      </dl>
    </article>
  );
}
