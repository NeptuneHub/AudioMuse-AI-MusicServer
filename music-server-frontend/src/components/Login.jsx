import React, { useState } from 'react';

const subsonicFetch = async (endpoint, creds, params = {}) => {
    const allParams = new URLSearchParams({
        u: creds.username, p: creds.password, v: '1.16.1', c: 'AudioMuse-AI', f: 'json', ...params
    });
    return fetch(`/rest/${endpoint}?${allParams.toString()}`);
};

function Login({ onLogin }) {
	const [username, setUsername] = useState('');
	const [password, setPassword] = useState('');
	const [error, setError] = useState('');

	const handleSubmit = async (e) => {
		e.preventDefault();
		setError('');
		try {
			const response = await subsonicFetch('getLicense.view', { username, password });

			if (!response.ok) {
				const data = await response.json();
				const subsonicResponse = data['subsonic-response'];
				setError(subsonicResponse?.error?.message || 'Login failed: Invalid credentials');
				return;
			}

			const data = await response.json();
			const subsonicResponse = data['subsonic-response'];

			if (subsonicResponse.status === 'ok') {
                // After successful Subsonic auth, get a JWT for the admin panel
                // and the user's actual admin status.
                const jwtResponse = await fetch('/api/v1/user/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ username, password })
                });

                if (!jwtResponse.ok) {
                    throw new Error('Failed to get JWT for admin panel');
                }

                const jwtData = await jwtResponse.json();
				onLogin({ username, password }, jwtData.is_admin, jwtData.token);
			} else {
				setError(subsonicResponse?.error?.message || 'Login failed');
			}
		} catch (err) {
			setError('Network error. Could not connect to server.');
            console.error(err);
		}
	};

	return (
		<div className="flex items-center justify-center h-screen">
			<div className="bg-gray-800 p-8 rounded-lg shadow-xl w-96">
				<h2 className="text-2xl font-bold mb-6 text-center text-teal-400">AudioMuse-AI</h2>
				<form onSubmit={handleSubmit}>
					<div className="mb-4">
						<label className="block text-gray-400 mb-2" htmlFor="username">Username</label>
						<input id="username" type="text" value={username} onChange={(e) => setUsername(e.target.value)} className="w-full p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500" />
					</div>
					<div className="mb-6">
						<label className="block text-gray-400 mb-2" htmlFor="password">Password</label>
						<input id="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} className="w-full p-2 bg-gray-700 rounded border border-gray-600 focus:outline-none focus:border-teal-500" />
					</div>
					{error && <p className="text-red-500 text-center mb-4">{error}</p>}
					<button type="submit" className="w-full bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded transition duration-300">
						Login
					</button>
				</form>
			</div>
		</div>
	);
}

export default Login;