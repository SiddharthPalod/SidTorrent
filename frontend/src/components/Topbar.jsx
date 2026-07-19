import React from 'react';

export default function Topbar({ healthOnline }) {
  return (
    <header className="my-4">
      {/* Top right status */}
      <div className="flex justify-end">
        <div className="flex items-center gap-2 text-xs uppercase text-[var(--muted)]">
          <span
            className={`w-3 h-3 rounded-full ${
              healthOnline
                ? "bg-[var(--acid)] shadow-[0_0_18px_var(--acid)]"
                : "bg-[var(--red)] shadow-[0_0_18px_var(--red)]"
            }`}
          />
          <span>{healthOnline ? "API ONLINE" : "API OFFLINE"}</span>
        </div>
      </div>

      {/* Large centered title */}
      <h1 className="mt-4 text-center text-8xl font-extrabold leading-none text-[var(--text)] animate-glow">
        SIDD TORRENT
      </h1>
    </header>
  );
}