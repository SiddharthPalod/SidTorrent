import React from 'react';

export default function LaunchForm({
  fileInputRef,
  fileLabel,
  handleFileChange,
  outputPath,
  setOutputPath,
  handleStart,
  isStarting,
}) {
  const folderInputRef = React.useRef(null);
  return (
    <form 
      className="flex flex-col md:flex-row items-end gap-5 p-5 border-b border-[var(--line)] w-full" 
      id="torrent-form" 
      onSubmit={handleStart}
    >
      <div className="flex flex-col gap-2 w-full md:w-auto md:flex-1">
        <span className="text-[0.78rem] uppercase text-[var(--muted)] font-semibold tracking-wider">
          Torrent Upload
        </span>
        <div className="flex items-center gap-3 w-full bg-[#071010] border border-[#33534e] rounded px-3 min-h-[44px]">
          <button 
            type="button"
            onClick={() => fileInputRef.current?.click()}
            className="px-3 py-1 font-mono font-bold text-[0.75rem]  text-[var(--acid)] border border-[var(--acid)] rounded hover:bg-[var(--cyan)]/15 transition cursor-pointer shrink-0"
          >
            📂 SELECT FILE
          </button>
          <span className="text-[var(--text)] text-xs truncate flex-1 font-mono">
            {fileLabel !== "No torrent uploaded." ? fileLabel : "No file selected"}
          </span>
          <input 
            ref={fileInputRef}
            id="torrent-file" 
            name="torrentFile" 
            type="file" 
            accept=".torrent,application/x-bittorrent" 
            onChange={handleFileChange}
            required 
            className="hidden"
          />
        </div>
      </div>

        <div className="flex flex-col gap-2 w-full md:w-auto md:flex-1">
          <label className="flex flex-col gap-2 text-[0.78rem] uppercase text-[var(--muted)] font-semibold tracking-wider w-full">
            <span>Output Folder</span>
            <div className="flex items-center gap-3 w-full bg-[#071010] border border-[#33534e] rounded px-3 min-h-[44px]">
              <button
                type="button"
                onClick={() => folderInputRef.current?.click()}
                className="px-3 py-1 font-mono font-bold text-[0.75rem] text-[var(--acid)] border border-[var(--acid)] rounded hover:bg-[var(--cyan)]/15 transition cursor-pointer shrink-0"
              >
                📂 SELECT FOLDER
              </button>
              <span className="text-[var(--text)] text-xs truncate flex-1 font-mono">
                {outputPath || 'No folder selected'}
              </span>
            </div>
            <input
              ref={folderInputRef}
              type="file"
              webkitdirectory="true"
              directory="true"
              hidden
              onChange={(e) => {
                const files = e.target.files;
                if (files.length > 0) {
                  const path = files[0].webkitRelativePath || files[0].name;
                  const folder = path.split('/')[0];
                  setOutputPath(folder);
                }
              }}
            />
          </label>
        </div>

      <div className="w-full md:w-auto">
        <button 
          type="submit"
          disabled={isStarting}
          className="w-full md:w-auto min-h-[44px] px-6 font-mono font-bold text-[0.85rem] bg-[var(--acid)] text-[#03100b] border border-[var(--acid)] rounded cursor-pointer transition hover:brightness-110 hover:shadow-[0_0_12px_rgba(56,255,156,0.3)] disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {isStarting ? 'Starting...' : 'Start Download'}
        </button>
      </div>
    </form>
  );
}
