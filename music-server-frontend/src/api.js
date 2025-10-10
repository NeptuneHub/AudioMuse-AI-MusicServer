// Minimal API helper: attach JWT from localStorage to requests when available
// Use an explicit API base URL in production. Set REACT_APP_API_URL at build time
// to override the default which assumes the backend is on the same host port 8080.
const DEFAULT_API_PORT = 8080;
const API_BASE = (process.env.REACT_APP_API_URL && process.env.REACT_APP_API_URL.replace(/"/g, '')) || `${window.location.protocol}//${window.location.hostname}:${DEFAULT_API_PORT}`;
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
