import React, { useState } from 'react';

function Login({ onLogin }) {
	const [username, setUsername] = useState('');
	const [password, setPassword] = useState('');
	const [error, setError] = useState('');
	const [isLoading, setIsLoading] = useState(false);

	const handleSubmit = async (e) => {
		e.preventDefault();
		setError('');
		setIsLoading(true);
		try {
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
		} finally {
			setIsLoading(false);
		}
	};

	return (
		<div className="gradient-animated flex items-center justify-center h-screen px-4 py-4 overflow-hidden">
			{/* Floating orbs for visual appeal - smaller */}
			<div className="absolute top-10 left-10 w-48 h-48 bg-accent-500 rounded-full mix-blend-multiply filter blur-3xl opacity-20 animate-pulse-slow"></div>
			<div className="absolute bottom-10 right-10 w-48 h-48 bg-purple-500 rounded-full mix-blend-multiply filter blur-3xl opacity-20 animate-pulse-slow animation-delay-2000"></div>
			
		<div className="relative z-10 w-full max-w-md animate-scale-in">
			{/* Glass card effect */}
			<div className="glass rounded-2xl shadow-2xl p-4 sm:p-6 backdrop-blur-xl">
				{/* Logo/Icon area - half size */}
				<div className="flex justify-center mb-4 -mt-2">
					<img src="/audiomuseai.png" alt="AudioMuse-AI" className="w-40 h-40 object-contain drop-shadow-lg" />
				</div>
				
				<form onSubmit={handleSubmit} className="space-y-3">
						<div>
							<label className="block text-gray-300 mb-1 font-medium text-sm" htmlFor="username">
								Username
							</label>
							<input 
								id="username" 
								type="text" 
								value={username} 
								onChange={(e) => setUsername(e.target.value)} 
								required
								disabled={isLoading}
								className="w-full p-2.5 bg-dark-700 rounded-lg border border-dark-600 focus:outline-none focus:border-accent-500 focus:ring-2 focus:ring-accent-500/20 text-white transition-all disabled:opacity-50" 
								placeholder="Enter your username"
							/>
						</div>
						<div>
							<label className="block text-gray-300 mb-1 font-medium text-sm" htmlFor="password">
								Password
							</label>
							<input 
								id="password" 
								type="password" 
								value={password} 
								onChange={(e) => setPassword(e.target.value)} 
								required
								disabled={isLoading}
								className="w-full p-2.5 bg-dark-700 rounded-lg border border-dark-600 focus:outline-none focus:border-accent-500 focus:ring-2 focus:ring-accent-500/20 text-white transition-all disabled:opacity-50" 
								placeholder="Enter your password"
							/>
						</div>
						
						{error && (
							<div className="bg-red-500/10 border border-red-500/50 rounded-lg p-2.5 animate-fade-in">
								<p className="text-red-400 text-center text-xs flex items-center justify-center gap-2">
									<svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
									</svg>
									{error}
								</p>
							</div>
						)}
						
						<button 
							type="submit" 
							disabled={isLoading}
							className="w-full bg-gradient-accent text-white font-bold py-2.5 px-4 rounded-lg transition-all duration-300 shadow-lg hover:shadow-glow-lg hover:scale-105 disabled:opacity-50 disabled:hover:scale-100 disabled:cursor-not-allowed flex items-center justify-center gap-2"
						>
							{isLoading ? (
								<>
									<svg className="animate-spin h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
										<circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
										<path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
									</svg>
									Signing in...
								</>
							) : (
								<>
									<svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M11 16l-4-4m0 0l4-4m-4 4h14m-5 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h7a3 3 0 013 3v1"></path>
									</svg>
									Sign In
								</>
							)}
						</button>
					</form>
				</div>
				
				{/* Bottom text */}
				<p className="text-center text-gray-500 mt-2 text-xs">
					Powered by{' '}
					<a 
						href="https://github.com/NeptuneHub/AudioMuse-AI" 
						target="_blank" 
						rel="noopener noreferrer"
						className="text-accent-400 hover:text-accent-300 transition-colors"
					>
						AudioMuse-AI Sonic Analysis
					</a>
				</p>
			</div>
		</div>
	);
}

export default Login;