import React from 'react';
import {
  FiGrid,
  FiRadio,
  FiDownloadCloud,
  FiLayers,
  FiPlayCircle,
} from 'react-icons/fi';

export default function Sidebar({ activeTab, setActiveTab, hasVideo }) {
  const tabs = [
    {
      id: 'deck',
      label: 'CONTROL DECK',
      icon: <FiGrid size={18} />,
    },
    {
      id: 'live',
      label: 'LIVE SESSION',
      icon: <FiRadio size={18} />,
    },
    {
      id: 'intel',
      label: 'TORRENT INTEL',
      icon: <FiDownloadCloud size={18} />,
    },
    {
      id: 'matrix',
      label: 'JOB MATRIX',
      icon: <FiLayers size={18} />,
    },
  ];

  if (hasVideo) {
    tabs.push({
      id: 'video',
      label: 'VIDEO PLAYER',
      icon: <FiPlayCircle size={18} />,
    });
  }

  return (
    <aside className="w-full lg:w-[220px] shrink-0 border-r-0 lg:border-r border-[var(--line)] bg-[#07090a]/40 flex flex-col font-mono">
      <div className="p-4 border-b border-[var(--line)] hidden lg:block">
        <span className="text-[0.7rem] text-[var(--muted)] uppercase tracking-widest block mb-1">
          Navigation
        </span>
        <h3 className="text-sm font-bold text-[var(--acid)] m-0">
          SWARM CONTROL
        </h3>
      </div>

      <nav className="flex flex-row lg:flex-col flex-wrap lg:flex-nowrap w-full">
        {tabs.map((tab) => {
          const isActive = activeTab === tab.id;

          return (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`sidebar-btn w-full text-left px-5 py-4 border-b border-[var(--line)] text-xs tracking-wider uppercase cursor-pointer min-h-[48px] ${
                isActive
                  ? 'active-nav-item'
                  : 'bg-transparent text-[var(--muted)]'
              }`}
            >
              <div className="flex items-center gap-3">
                <span className="shrink-0">{tab.icon}</span>
                <span>{tab.label}</span>
              </div>
            </button>
          );
        })}
      </nav>
    </aside>
  );
}