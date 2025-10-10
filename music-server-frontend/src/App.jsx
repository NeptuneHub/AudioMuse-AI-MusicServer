// Suggested path: music-server-frontend/src/App.jsx
import React, { useState, useEffect } from 'react';
import Dashboard from './components/Dashboard';
import Login from './components/Login';

// --- Main App Component ---
function App() {
	// Subsonic auth is session-based; we store credentials for the session.
	const [credentials, setCredentials] = useState(null);
	const [isAdmin, setIsAdmin] = useState(false);

	// Restore session from localStorage (JWT + username) on app load
	// Only restore if we have a valid JWT token
	useEffect(() => {
		const validateAndRestoreSession = async () => {
			const token = localStorage.getItem('token');
			const username = localStorage.getItem('username');
			const storedAdminStatus = localStorage.getItem('isAdmin') === 'true';
			
			if (token && username) {
				try {
					// Validate token by making a test API call
					const { apiFetch } = await import('./api');
					const response = await apiFetch('/api/v1/user/me');
					
					if (response.ok) {
						// Token is valid, restore session
						setCredentials({ username, password: null });
						setIsAdmin(storedAdminStatus);
					} else {
						// Token is invalid, clear storage
						localStorage.removeItem('token');
						localStorage.removeItem('username'); 
						localStorage.removeItem('isAdmin');
					}
				} catch (error) {
					// Network error or token validation failed, clear storage
					console.error('Session validation failed:', error);
					localStorage.removeItem('token');
					localStorage.removeItem('username'); 
					localStorage.removeItem('isAdmin');
				}
			}
		};
		
		validateAndRestoreSession();
	}, []);

	const handleLogin = (creds, adminStatus, token) => {
		// If we received a JWT token, prefer storing only the username in credentials
		// to avoid sending plaintext passwords in querystrings from the UI.
		if (token) {
			setCredentials({ username: creds.username, password: null });
		} else {
			setCredentials(creds);
		}
		setIsAdmin(adminStatus);
		if (token) {
			localStorage.setItem('token', token); // Store the JWT for admin actions
		}
		if (creds && creds.username) {
			localStorage.setItem('username', creds.username);
		}
		// Persist admin status so it survives page reload
		localStorage.setItem('isAdmin', adminStatus.toString());
	};

	const handleLogout = () => {
		localStorage.removeItem('token'); // For admin panel
		localStorage.removeItem('username');
		localStorage.removeItem('isAdmin');
		setCredentials(null);
		setIsAdmin(false);
	};

	return (
		<div className="bg-gray-900 text-white min-h-screen font-sans">
			{credentials ? (
				<Dashboard onLogout={handleLogout} isAdmin={isAdmin} credentials={credentials} />
			) : (
				<Login onLogin={handleLogin} />
			)}
		</div>
	);
}

export default App;
