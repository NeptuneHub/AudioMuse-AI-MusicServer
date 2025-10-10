import React, { useState } from 'react';

function Login({ onLogin }) {
	const [username, setUsername] = useState('');
	const [password, setPassword] = useState('');
	const [error, setError] = useState('');

	const handleSubmit = async (e) => {
		e.preventDefault();
		setError('');
		try {
			// REVERT TO WORKING COMMIT APPROACH - Direct fetch like fa38d3de9
			const jwtResponse = await fetch('/api/v1/user/login', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ username, password })
			});

			if (jwtResponse.ok) {
				const jwtData = await jwtResponse.json();
				localStorage.setItem('username', username);
				localStorage.setItem('token', jwtData.token);
				onLogin({ username, password: null }, jwtData.is_admin, jwtData.token);
				return;
			} else {
				const errorData = await jwtResponse.json().catch(() => ({}));
				setError(errorData.error || 'Login failed: Invalid credentials');
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