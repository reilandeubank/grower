// Service worker runtime caches (vite.config.ts). Logout deletes them, so the next person to
// sign in on the device can't read the last one's plants offline.
export const API_CACHE = "api";
export const MEDIA_CACHE = "media";
