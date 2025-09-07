// Suggested path: music-server-frontend/src/App.jsx
import React, { useState } from 'react';
import Dashboard from './components/Dashboard';
import Login from './components/Login';

// --- Main App Component ---
function App() {
	// Subsonic auth is session-based; we store credentials for the session.
	const [credentials, setCredentials] = useState(null);
	const [isAdmin, setIsAdmin] = useState(false);

	const handleLogin = (creds, adminStatus) => {
		setCredentials(creds);
		setIsAdmin(adminStatus);
	};

	const handleLogout = () => {
		localStorage.removeItem('token'); // For admin panel
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
