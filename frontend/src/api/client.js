// API client helpers for interacting with SiddTorrent API

export const api = async (path, options = {}) => {
  const response = await fetch(path, options);
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.error || `Request failed: ${response.status}`);
  }
  return payload;
};

export const jsonApi = (path, options = {}) =>
  api(path, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });

export const formatBytes = (value) => {
  if (!value) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let index = 0;
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024;
    index += 1;
  }
  return `${size.toFixed(index === 0 ? 0 : 2)} ${units[index]}`;
};

export const getJobs = () => api("/api/downloads");

export const checkHealth = () => api("/api/health");

export const uploadTorrent = (file) => {
  const formData = new FormData();
  formData.append("torrent", file);
  return api("/api/torrents/upload", {
    method: "POST",
    body: formData,
  });
};

export const startDownload = (payload) => {
  return jsonApi("/api/downloads", {
    method: "POST",
    body: JSON.stringify(payload),
  });
};

export const cancelDownload = (jobId) => {
  return jsonApi(`/api/downloads/${jobId}`, {
    method: "DELETE",
  });
};
