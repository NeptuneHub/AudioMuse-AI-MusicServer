// REVERT TO WORKING APPROACH FROM fa38d3de9 COMMIT
// Both frontend and backend are in same container
// Use relative URLs - no explicit API_BASE needed
// Container internal routing handles the rest

const API_BASE = ''; // Empty = relative URLs like the working version

export { API_BASE };

export async function apiFetch(path, options = {}) {
    const headers = options.headers || {};
    const token = localStorage.getItem('token');
    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    } else if (path !== '/api/v1/user/login') {
        console.warn('apiFetch: No JWT token found for:', path);
    }
    // Prevent cached 304 responses which can return empty bodies and break JSON parsing
    headers['Cache-Control'] = 'no-cache';
    headers['Pragma'] = 'no-cache';
    if (!headers['Content-Type'] && !(options.body instanceof FormData)) {
        headers['Content-Type'] = 'application/json';
    }
    const finalOptions = { ...options, headers };
    const url = path.startsWith('http') ? path : `${API_BASE}${path}`;
    console.debug('apiFetch:', { path, url, method: options.method || 'GET', hasToken: !!token });
    const res = await fetch(url, finalOptions);
    if (!res.ok) {
        console.error('apiFetch failed:', { path, status: res.status, statusText: res.statusText });
        
        // Handle 401 Unauthorized - likely expired JWT token (but not for login endpoint)
        if (res.status === 401 && path !== '/api/v1/user/login') {
            console.warn('JWT token expired or invalid, clearing session');
            localStorage.removeItem('token');
            localStorage.removeItem('username');
            localStorage.removeItem('isAdmin');
            // Reload page to force login
            window.location.reload();
            return res;
        }
    }
    return res;
}

// For Subsonic endpoints - JWT-ONLY authentication (no legacy fallback)
export async function subsonicFetch(endpoint, params = {}) {
    const allParams = new URLSearchParams({
        v: '1.16.1', c: 'AudioMuse-AI', f: 'json', ...params
    });
    
    // FRONTEND USES JWT ONLY - no username/password in querystring
    const token = localStorage.getItem('token');
    const headers = {};
    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    } else {
        console.error('No JWT token found for subsonicFetch call to:', endpoint);
        throw new Error('No JWT token found. Please log in again.');
    }

    const url = `${API_BASE}/rest/${endpoint}?${allParams.toString()}`;
    console.debug('subsonicFetch:', { endpoint, url, hasToken: !!token });
    
    const res = await fetch(url, { headers });
    if (!res.ok) {
        console.error('subsonicFetch failed:', { endpoint, status: res.status, statusText: res.statusText });
        
        // Handle 401 Unauthorized - likely expired JWT token
        if (res.status === 401) {
            console.warn('JWT token expired or invalid, clearing session');
            localStorage.removeItem('token');
            localStorage.removeItem('username');
            localStorage.removeItem('isAdmin');
            // Reload page to force login
            window.location.reload();
            return;
        }
        
        // Attempt to parse JSON error body, otherwise throw generic
        try {
            const data = await res.json();
            const subsonicResponse = data['subsonic-response'];
            const errorMsg = subsonicResponse?.error?.message || `Server error: ${res.status}`;
            console.error('subsonicFetch error details:', subsonicResponse?.error);
            throw new Error(errorMsg);
        } catch (e) {
            throw new Error(`Server error: ${res.status} - ${res.statusText}`);
        }
    }
    const data = await res.json();
    return data['subsonic-response'];
}

// Star/Unstar functions
export async function starSong(songId) {
    return await subsonicFetch('star.view', { id: songId });
}

export async function unstarSong(songId) {
    return await subsonicFetch('unstar.view', { id: songId });
}

export async function getStarredSongs() {
    return await subsonicFetch('getStarred.view');
}

export async function starAlbum(albumId) {
    return await subsonicFetch('star.view', { albumId });
}

export async function unstarAlbum(albumId) {
    return await subsonicFetch('unstar.view', { albumId });
}

export async function starArtist(artistId) {
    return await subsonicFetch('star.view', { artistId });
}

export async function unstarArtist(artistId) {
    return await subsonicFetch('unstar.view', { artistId });
}

// Genre functions
export async function getGenres() {
    return await subsonicFetch('getGenres.view');
}

export async function getSongsByGenre(genre, params = {}) {
    return await subsonicFetch('getSongsByGenre.view', { genre, ...params });
}

// Enhanced search with genre support
export async function searchMusic(query, params = {}) {
    return await subsonicFetch('search2.view', { query, ...params });
}

// Enhanced album list with genre filtering
export async function getAlbums(params = {}) {
    return await subsonicFetch('getAlbumList2.view', { type: 'alphabeticalByArtist', ...params });
}

// Rescan functionality for admin
export async function rescanLibrary() {
    return await apiFetch('/api/v1/admin/scan/rescan', { method: 'POST' });
}

// Discovery views
export async function getMusicCounts(genre = '') {
    const params = genre ? `?genre=${encodeURIComponent(genre)}` : '';
    const res = await apiFetch(`/api/v1/counts${params}`);
    return await res.json();
}

export async function getRecentlyAdded(limit = 50, offset = 0, genre = '') {
    const params = new URLSearchParams({ limit, offset });
    if (genre) params.set('genre', genre);
    const res = await apiFetch(`/api/v1/recently-added?${params.toString()}`);
    const data = await res.json();
    return Array.isArray(data) ? data : [];
}

export async function getMostPlayed(limit = 50, offset = 0, genre = '') {
    const params = new URLSearchParams({ limit, offset });
    if (genre) params.set('genre', genre);
    const res = await apiFetch(`/api/v1/most-played?${params.toString()}`);
    const data = await res.json();
    return Array.isArray(data) ? data : [];
}

export async function getRecentlyPlayed(limit = 50, offset = 0, genre = '') {
    const params = new URLSearchParams({ limit, offset });
    if (genre) params.set('genre', genre);
    const res = await apiFetch(`/api/v1/recently-played?${params.toString()}`);
    const data = await res.json();
    return Array.isArray(data) ? data : [];
}

// Radio API functions
export async function createRadio(name, seedSongs, temperature, subtractDistance) {
    const res = await apiFetch('/api/radios', {
        method: 'POST',
        body: JSON.stringify({
            name,
            seed_songs: JSON.stringify(seedSongs),
            temperature,
            subtract_distance: subtractDistance
        })
    });
    return await res.json();
}

export async function getRadios() {
    const res = await apiFetch('/api/radios');
    return await res.json();
}

export async function getRadioSeed(radioId) {
    const res = await apiFetch(`/api/radios/${radioId}/seed`);
    return await res.json();
}

export async function deleteRadio(radioId) {
    const res = await apiFetch(`/api/radios/${radioId}`, { method: 'DELETE' });
    return await res.json();
}

export async function updateRadioName(radioId, name) {
    const res = await apiFetch(`/api/radios/${radioId}/name`, {
        method: 'PUT',
        body: JSON.stringify({ name })
    });
    return await res.json();
}

// Similar artists function
export async function getSimilarArtists(artistId, count = 20) {
    return await subsonicFetch('getSimilarArtists2.view', { id: artistId, count });
}

// CLAP search functions
export async function clapSearch(query, limit = 50) {
    const res = await apiFetch('/api/clap/search', {
        method: 'POST',
        body: JSON.stringify({ query, limit })
    });
    return await res.json();
}

export async function getClapTopQueries() {
    const res = await apiFetch('/api/clap/top_queries');
    return await res.json();
}

