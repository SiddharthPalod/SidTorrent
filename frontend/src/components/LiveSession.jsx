import React from 'react';

export default function LiveSession({
  percent,
  phase,
  message,
  isCancelDisabled,
  handleCancel,
}) {
  const clamped = Math.max(0, Math.min(100, Number(percent || 0)));

  return (
    <article className="flex-1 md:flex-[0.5] min-w-[200px] border border-[var(--line)] rounded bg-[var(--panel)]">
      <div className="flex justify-between items-center gap-3.5 min-h-[56px] px-4 border-b border-[var(--line)]">
        <h2 className="text-base  text-[var(--acid)] m-0 font-bold">Live Session</h2>
        <button 
          className="bg-transparent border border-[var(--acid)] rounded text-[var(--acid)] px-4 py-1.5 font-bold font-mono text-[0.8rem] cursor-pointer hover:brightness-110 disabled:opacity-40 disabled:cursor-not-allowed transition" 
          id="cancel-btn" 
          type="button" 
          disabled={isCancelDisabled}
          onClick={handleCancel}
        >
          Cancel
        </button>
      </div>

      <div className="w-full p-2 flex flex-col gap-1 shrink-0 bg-[#040708]/30">
        <div className="flex justify-between items-center text-xs font-mono font-bold">
          <span className="uppercase text-[var(--muted)]">Download Progress</span>
          <span id="percent" className="text-[var(--acid)] text-sm">{clamped.toFixed(2)}%</span>
        </div>
        <div className="w-full h-5 bg-[#101617] border border-[var(--line)] rounded overflow-hidden relative">
          <div 
            className="h-full bg-[var(--acid)] transition-[width] duration-300 shadow-[0_0_8px_var(--acid)]"
            style={{ width: `${clamped}%` }}
          />
        </div>
      </div>

      <div className="px-5 py-6 text-center flex flex-col items-center">
        <span id="phase" className="block mb-2.5 text-[var(--amber)] uppercase tracking-wider text-[0.9rem] font-bold">
          {phase}
        </span>
        <p id="message" className="text-[var(--muted)] m-0 leading-relaxed text-[0.85rem] font-mono">
          {message}
        </p>
      </div>
    </article>
  );
}
