// Centralized token management
export function getAuthToken() {
    return localStorage.getItem('token');
}

export function setAuthToken(token) {
    if (token) {
        localStorage.setItem('token', token);
    }
}

export function clearAuthToken() {
    localStorage.removeItem('token');
}

export function getAuthHeaders() {
    const token = getAuthToken();
    return token ? { 'Authorization': `Bearer ${token}` } : {};
}
