// Suggested path: music-server-frontend/src/components/AdminPanel.jsx
import React, { useState, useEffect, useCallback, useRef } from 'react';

const Modal = ({ children, onClose }) => (
    <div className="fixed inset-0 bg-black bg-opacity-60 flex items-center justify-center z-50 p-4">
        <div className="bg-gray-800 p-6 rounded-lg shadow-xl w-full sm:w-11/12 md:max-w-md relative">
            <button onClick={onClose} className="absolute top-2 right-2 text-gray-400 hover:text-white">
                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path></svg>
            </button>
            {children}
        </div>
    </div>
);

function FileBrowser({ onSelect, onClose }) {
    const [currentPath, setCurrentPath] = useState('/');
    const [items, setItems] = useState([]);
    const [error, setError] = useState('');

    const fetchDirectory = useCallback(async (path) => {
        setError('');
        const token = localStorage.getItem('token');
        try {
            const response = await fetch(`/api/v1/admin/browse?path=${encodeURIComponent(path)}`, {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || `Server error: ${response.status}`);
            }
            const data = await response.json();
            let dirItems = (data.items || []).filter(i => i.type === 'dir');
            
            const isWindowsRoot = /^[a-zA-Z]:\\?$/.test(data.path);
            const isUnixRoot = data.path === '/';

            if (!isUnixRoot && !isWindowsRoot) {
                 if (!dirItems.some(i => i.name === '..')) {
                    dirItems.unshift({ name: '..', type: 'dir' });
                }
            }
            setItems(dirItems);
            setCurrentPath(data.path || path);
        } catch (err) {
            setError(err.message);
            setItems([]);
        }
    }, []);

    useEffect(() => {
        fetchDirectory('/');
    }, [fetchDirectory]);

    const handleItemClick = (item) => {
        let newPath;
        const isWindows = currentPath.includes('\\');
        const separator = isWindows ? '\\' : '/';

        if (item.name === '..') {
            const parts = currentPath.split(separator).filter(p => p && p !== ':');
            parts.pop();
            if (isWindows) {
                if (parts.length === 1 && parts[0].endsWith(':')) {
                    newPath = parts[0] + separator;
                } else if (parts.length === 0) {
                    newPath = '/';
                }
                else {
                    newPath = parts.join(separator);
                }
            } else {
                newPath = separator + parts.join(separator);
            }
        } else {
            if (currentPath.endsWith(separator)) {
                newPath = `${currentPath}${item.name}`;
            } else {
                newPath = `${currentPath}${separator}${item.name}`;
            }
        }
        fetchDirectory(newPath);
    };

    return (
        <Modal onClose={onClose}>
             <h3 className="text-xl font-bold mb-4 text-teal-400">Browse For Folder</h3>
             <div className="bg-gray-900 p-2 rounded mb-4 font-mono text-sm break-all">{currentPath}</div>
             {error && <p className="text-red-500 mb-4">Error: {error}</p>}
             <ul className="h-64 overflow-y-auto border border-gray-700 rounded p-2 mb-4">
                 {items.map((item, index) => (
                     <li key={index} onClick={() => handleItemClick(item)} className="p-2 hover:bg-gray-700 rounded cursor-pointer flex items-center"><svg className="w-5 h-5 mr-2 text-teal-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"></path></svg>{item.name}</li>
                 ))}
             </ul>
             <div className="flex flex-col sm:flex-row justify-end space-y-2 sm:space-y-0 sm:space-x-4">
                 <button onClick={onClose} className="bg-gray-600 hover:bg-gray-700 text-white font-bold py-2 px-4 rounded">Cancel</button>
                 <button onClick={() => onSelect(currentPath)} className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Select Folder</button>
             </div>
        </Modal>
    );
}

const LibraryPathModal = ({ path, onClose, onSave }) => {
    const [currentPath, setCurrentPath] = useState(path ? path.path : '');
    const [showBrowser, setShowBrowser] = useState(false);

    const handleSave = () => {
        onSave({ ...path, path: currentPath });
    };

    return (
        <Modal onClose={onClose}>
            <h3 className="text-xl font-bold mb-4 text-teal-400">{path ? 'Edit Library Path' : 'Add Library Path'}</h3>
            <div className="flex space-x-2">
                <input
                    type="text"
                    value={currentPath}
                    placeholder="Enter or browse for a folder..."
                    className="flex-grow p-2 bg-gray-700 rounded border border-gray-600"
                    readOnly
                />
                <button onClick={() => setShowBrowser(true)} className="bg-blue-600 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded">Browse</button>
            </div>
            <div className="flex justify-end space-x-4 mt-6">
                <button onClick={onClose} className="bg-gray-600 hover:bg-gray-700 text-white font-bold py-2 px-4 rounded">Cancel</button>
                <button onClick={handleSave} className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Save</button>
            </div>
            {showBrowser && <FileBrowser
				onSelect={(selectedPath) => { setCurrentPath(selectedPath); setShowBrowser(false); }}
				onClose={() => setShowBrowser(false)}
			/>}
        </Modal>
    );
};


function LibraryManagement({ onConfigChange }) {
    const [scanStatus, setScanStatus] = useState({ scanning: false, count: 0 });
    const [message, setMessage] = useState('');
    const [libraryPaths, setLibraryPaths] = useState([]);
    const [editingPath, setEditingPath] = useState(null);
    const [isAddingPath, setIsAddingPath] = useState(false);
    const [error, setError] = useState('');
    const wasScanningRef = useRef(false);

    useEffect(() => {
        wasScanningRef.current = scanStatus.scanning;
    });

    const subsonicApiRequest = useCallback(async (method, endpoint, body = null) => {
        const token = localStorage.getItem('token');
        const options = {
            method,
            headers: { 'Authorization': `Bearer ${token}` }
        };
         const url = new URL(`/rest/${endpoint}`, window.location.origin);
        url.searchParams.append('f', 'json');

        if (method === 'GET' && body) {
            Object.entries(body).forEach(([key, value]) => url.searchParams.append(key, value));
        } else if (body) {
            options.headers['Content-Type'] = 'application/json';
            options.body = JSON.stringify(body);
        }
        
        const response = await fetch(url, options);
        const data = await response.json();

        if (!response.ok || data?.["subsonic-response"]?.status === 'failed') {
            const error = data?.["subsonic-response"]?.error;
            throw new Error(error?.message || `Server error: ${response.status}`);
        }
        
        return data["subsonic-response"];
    }, []);

    const fetchLibraryPaths = useCallback(async () => {
        try {
            const data = await subsonicApiRequest('GET', 'getLibraryPaths.view');
            const paths = data?.libraryPaths?.path || [];
            setLibraryPaths(Array.isArray(paths) ? paths : [paths].filter(Boolean));
        } catch (err) {
            setError(err.message || 'Failed to fetch library paths');
        }
    }, [subsonicApiRequest]);

    const fetchStatus = useCallback(async (isInitialFetch = false) => {
        try {
            const data = await subsonicApiRequest('GET', 'getScanStatus.view');
            if (data && data.scanStatus) {
                const isScanningNow = data.scanStatus.scanning;
                setScanStatus({ scanning: isScanningNow, count: data.scanStatus.count });
                if (wasScanningRef.current && !isScanningNow) {
                    fetchLibraryPaths();
                }
            } else {
                 throw new Error('Invalid response from server.');
            }
        } catch (e) {
            if (!isInitialFetch) {
                setMessage(`Error fetching scan status: ${e.message}`);
            } else {
                console.error("Initial scan status fetch failed. This may be normal.", e);
            }
        }
    }, [subsonicApiRequest, fetchLibraryPaths]);

    useEffect(() => {
      fetchLibraryPaths();
      fetchStatus(true);
    }, [fetchLibraryPaths, fetchStatus]);

    useEffect(() => {
        if (!scanStatus.scanning) return;

        const intervalId = setInterval(() => fetchStatus(false), 3000);
        return () => clearInterval(intervalId);
    }, [scanStatus.scanning, fetchStatus]);

    const handleStartScan = async (pathId = null) => {
        setMessage('');
        setError('');
        setScanStatus(prev => ({ ...prev, scanning: true, count: 0 }));
        try {
            const params = pathId ? { pathId } : {};
            await subsonicApiRequest('GET', 'startScan.view', params);
        } catch (e) {
            setScanStatus(prev => ({ ...prev, scanning: false }));
            setError(e.message || 'Error starting scan.');
        }
    };
    
    const handleSavePath = async (pathData) => {
        setError('');
        try {
            const endpoint = pathData.id ? 'updateLibraryPath.view' : 'addLibraryPath.view';
            const data = await subsonicApiRequest('POST', endpoint, pathData);
            const paths = data?.libraryPaths?.path || [];
            setLibraryPaths(Array.isArray(paths) ? paths : [paths].filter(Boolean));
        } catch (err) {
            setError(err.message);
        } finally {
            setEditingPath(null);
            setIsAddingPath(false);
        }
    };

    const handleDeletePath = async (pathId) => {
        if (!window.confirm("Are you sure you want to delete this library path? This won't delete the files.")) return;
        setError('');
        try {
            const data = await subsonicApiRequest('POST', 'deleteLibraryPath.view', { id: pathId });
            const paths = data?.libraryPaths?.path || [];
            setLibraryPaths(Array.isArray(paths) ? paths : [paths].filter(Boolean));
        } catch (err) {
            setError(err.message);
        }
    };

    const handleCancelScan = async () => {
        setMessage('Cancelling scan...');
        try {
            const token = localStorage.getItem('token');
            const response = await fetch('/api/v1/admin/scan/cancel', {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || 'Failed to cancel scan');
            }
            setMessage('Cancellation signal sent. Scan will stop shortly.');
        } catch (e) {
            setMessage(e.message);
        }
    };

    const formatDate = (isoString) => {
        if (!isoString) return 'Never';
        try {
            return new Date(isoString).toLocaleString();
        } catch (e) {
            return 'Invalid Date';
        }
    };

    return (
        <div className="bg-gray-800 p-4 sm:p-6 rounded-lg">
            <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center mb-4 space-y-2 sm:space-y-0">
                <h3 className="text-xl font-bold">Library Management</h3>
                <div>
                     <button onClick={() => handleStartScan(null)} disabled={scanStatus.scanning} className="bg-green-600 hover:bg-green-700 text-white font-bold py-2 px-4 rounded disabled:bg-green-400 disabled:cursor-not-allowed mr-2">
                        Scan All
                    </button>
                    <button onClick={() => setIsAddingPath(true)} disabled={scanStatus.scanning} className="bg-indigo-600 hover:bg-indigo-700 text-white font-bold py-2 px-4 rounded disabled:bg-indigo-400 disabled:cursor-not-allowed">
                        Add Path
                    </button>
                </div>
            </div>

            {error && <p className="text-red-500 mb-4 p-3 bg-red-900/50 rounded">{error}</p>}
            {message && !scanStatus.scanning && <p className="text-sm text-center mb-2 p-3 bg-gray-700 rounded">{message}</p>}

            {scanStatus.scanning && (
                <div className="text-center my-4 p-3 bg-gray-700 rounded">
                    <p>Scan in Progress... {scanStatus.count} new songs found.</p>
                     <button onClick={handleCancelScan} className="mt-2 bg-red-600 hover:bg-red-700 text-white font-bold py-1 px-3 rounded text-sm">
                        Cancel Scan
                    </button>
                </div>
            )}
            
             <div className="overflow-x-auto">
                <table className="min-w-full text-sm text-left text-gray-400">
                    <thead className="text-xs text-gray-300 uppercase bg-gray-700">
                        <tr>
                            <th scope="col" className="px-6 py-3">Path</th>
                            <th scope="col" className="px-6 py-3">Songs</th>
                            <th scope="col" className="px-6 py-3">Last Scanned</th>
                            <th scope="col" className="px-6 py-3 text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {libraryPaths.map(path => (
                            <tr key={path.id} className="bg-gray-800 border-b border-gray-700 hover:bg-gray-600">
                                <td className="px-6 py-4 font-mono text-white break-all">{path.path}</td>
                                <td className="px-6 py-4">{path.songCount}</td>
                                <td className="px-6 py-4">{formatDate(path.lastScanEnded)}</td>
                                <td className="px-6 py-4 text-right space-x-2 whitespace-nowrap">
                                    <button onClick={() => handleStartScan(path.id)} disabled={scanStatus.scanning} className="font-medium text-green-500 hover:underline disabled:text-gray-500 disabled:cursor-not-allowed">Scan</button>
                                    <button onClick={() => setEditingPath(path)} disabled={scanStatus.scanning} className="font-medium text-blue-500 hover:underline disabled:text-gray-500 disabled:cursor-not-allowed">Edit</button>
                                    <button onClick={() => handleDeletePath(path.id)} disabled={scanStatus.scanning} className="font-medium text-red-500 hover:underline disabled:text-gray-500 disabled:cursor-not-allowed">Delete</button>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>

            {(isAddingPath || editingPath) && (
                <LibraryPathModal
                    path={editingPath}
                    onClose={() => { setIsAddingPath(false); setEditingPath(null); }}
                    onSave={handleSavePath}
                />
            )}
        </div>
    );
}

function AutoScanManagement({ onConfigChange }) {
    const [schedule, setSchedule] = useState('');
    const [isEnabled, setIsEnabled] = useState(false);
    const [message, setMessage] = useState('');
    const [error, setError] = useState('');

    const subsonicApiRequest = useCallback(async (endpoint, params = {}) => {
        const token = localStorage.getItem('token');
        const query = new URLSearchParams(params);
        query.append('f', 'json');

        const response = await fetch(`/rest/${endpoint}?${query.toString()}`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const data = await response.json();
        const subsonicResponse = data["subsonic-response"];

        if (!response.ok || subsonicResponse.status === 'failed') {
            const error = subsonicResponse?.error;
            throw new Error(error?.message || `Server error: ${response.status}`);
        }
        return subsonicResponse;
    }, []);

    const fetchConfig = useCallback(async () => {
        try {
            const data = await subsonicApiRequest('getConfiguration.view');
            const configList = data?.configurations?.configuration || [];
            const allConfigs = Array.isArray(configList) ? configList : [configList].filter(Boolean);
            
            const scheduleConfig = allConfigs.find(c => c.name === 'scan_schedule');
            setSchedule(scheduleConfig?.value || '0 2 * * *');

            const enabledConfig = allConfigs.find(c => c.name === 'scan_enabled');
            setIsEnabled(enabledConfig?.value === 'true');

        } catch (err) {
            setError(err.message || 'Failed to fetch scan configuration');
        }
    }, [subsonicApiRequest]);

    useEffect(() => {
        fetchConfig();
    }, [fetchConfig]);

    const handleSave = async () => {
        setError('');
        setMessage('');
        try {
            // Save schedule and enabled status as two separate API calls
            await subsonicApiRequest('setConfiguration.view', { key: 'scan_schedule', value: schedule });
            await subsonicApiRequest('setConfiguration.view', { key: 'scan_enabled', value: isEnabled });
            
            setMessage('Auto-scan settings saved successfully! Restart the server for changes to take effect.');
            onConfigChange();
            setTimeout(() => setMessage(''), 5000);
        } catch (err) {
            setError(err.message || 'Failed to save settings.');
        }
    };

    return (
        <div className="bg-gray-800 p-4 sm:p-6 rounded-lg">
            <h3 className="text-xl font-bold mb-4">Automatic Library Scanning</h3>
            <div className="space-y-4">
                <div className="flex items-center">
                    <input
                        type="checkbox"
                        id="scan-enabled"
                        checked={isEnabled}
                        onChange={(e) => setIsEnabled(e.target.checked)}
                        className="w-5 h-5 text-teal-600 bg-gray-700 border-gray-600 rounded focus:ring-teal-500"
                    />
                    <label htmlFor="scan-enabled" className="ml-3 text-sm font-medium text-gray-300">Enable automatic scanning</label>
                </div>
                <div>
                    <label htmlFor="scan-schedule" className="block text-sm font-medium text-gray-300">Cron Schedule</label>
                     <input
                        type="text"
                        id="scan-schedule"
                        value={schedule}
                        onChange={(e) => setSchedule(e.target.value)}
                        placeholder="0 2 * * *"
                        className="w-full p-2 mt-1 bg-gray-700 rounded border border-gray-600 font-mono"
                        disabled={!isEnabled}
                    />
                    <p className="text-xs text-gray-400 mt-1">Standard cron format. Default is '0 2 * * *' (2 AM daily).</p>
                </div>
            </div>
             <div className="mt-4 flex justify-end">
                <button onClick={handleSave} className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Save Settings</button>
            </div>
            {message && <p className="text-green-400 mt-2">{message}</p>}
            {error && <p className="text-red-500 mt-2">{error}</p>}
        </div>
    );
}


function AIConfigManagement({ onConfigChange }) {
    const [audiomuseUrl, setAudiomuseUrl] = useState('');
    const [message, setMessage] = useState('');
    const [error, setError] = useState('');

    const subsonicApiRequest = useCallback(async (endpoint, params = {}) => {
        const token = localStorage.getItem('token');
        const query = new URLSearchParams(params);
        query.append('f', 'json');
        const response = await fetch(`/rest/${endpoint}?${query.toString()}`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const data = await response.json();
        const subsonicResponse = data["subsonic-response"];
        if (!response.ok || subsonicResponse.status === 'failed') {
            const error = subsonicResponse?.error;
            throw new Error(error?.message || `Server error: ${response.status}`);
        }
        return subsonicResponse;
    }, []);

    const fetchConfig = useCallback(async () => {
        try {
            const data = await subsonicApiRequest('getConfiguration.view');
            const configList = data?.configurations?.configuration || [];
            const allConfigs = Array.isArray(configList) ? configList : [configList].filter(Boolean);
            const urlConfig = allConfigs.find(c => c.name === 'audiomuse_ai_core_url');
            setAudiomuseUrl(urlConfig?.value || '');
        } catch (err) {
            setError(err.message || 'Failed to fetch configuration');
        }
    }, [subsonicApiRequest]);

    useEffect(() => {
        fetchConfig();
    }, [fetchConfig]);

    const handleSave = async () => {
        setError('');
        setMessage('');
        try {
            await subsonicApiRequest('setConfiguration.view', { key: 'audiomuse_ai_core_url', value: audiomuseUrl });
            setMessage('URL saved successfully!');
            onConfigChange();
            setTimeout(() => setMessage(''), 3000);
        } catch (err) {
            setError(err.message || 'Failed to save URL.');
        }
    };

    return (
        <div className="bg-gray-800 p-4 sm:p-6 rounded-lg">
            <h3 className="text-xl font-bold mb-4">AudioMuse-AI Core Integration</h3>
            <div className="space-y-2">
                <label htmlFor="audiomuse-url" className="block text-sm font-medium text-gray-300">Core Service URL</label>
                <input
                    type="text"
                    id="audiomuse-url"
                    value={audiomuseUrl}
                    onChange={(e) => setAudiomuseUrl(e.target.value)}
                    placeholder="http://localhost:8000"
                    className="w-full p-2 bg-gray-700 rounded border border-gray-600"
                />
            </div>
            <div className="mt-4 flex justify-end">
                <button onClick={handleSave} className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Save</button>
            </div>
            {message && <p className="text-green-400 mt-2">{message}</p>}
            {error && <p className="text-red-500 mt-2">{error}</p>}
        </div>
    );
}

function UserManagement() {
	const [users, setUsers] = useState([]);
	const [editingUser, setEditingUser] = useState(null);
    const [isCreatingUser, setIsCreatingUser] = useState(false);
	const [error, setError] = useState('');
    const [successMessage, setSuccessMessage] = useState('');

    const subsonicApiRequest = useCallback(async (endpoint, params = {}) => {
        const token = localStorage.getItem('token');
        const query = new URLSearchParams(params);
        query.append('f', 'json');

        const response = await fetch(`/rest/${endpoint}?${query.toString()}`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const data = await response.json();
        const subsonicResponse = data["subsonic-response"];

        if (!response.ok || subsonicResponse.status === 'failed') {
            const error = subsonicResponse?.error;
            throw new Error(error?.message || `Server error: ${response.status}`);
        }
        return subsonicResponse;
    }, []);

	const fetchUsers = useCallback(async () => {
		try {
			const data = await subsonicApiRequest('getUsers.view');
			const userList = data?.users?.user || [];
			setUsers(userList.map(u => ({ username: u.username, is_admin: u.adminRole })));
		} catch (err) {
			setError(err.message || 'Failed to fetch users');
		}
	}, [subsonicApiRequest]);

	useEffect(() => {
		fetchUsers();
	}, [fetchUsers]);

    const handleCreate = async (userData) => {
        setError('');
        setSuccessMessage('');
        try {
            await subsonicApiRequest('createUser.view', {
                username: userData.username,
                password: userData.password,
                adminRole: userData.is_admin,
            });
            setIsCreatingUser(false);
            setSuccessMessage(`User ${userData.username} created successfully.`);
            fetchUsers();
        } catch (err) {
            setError(err.message || 'Failed to create user.');
        }
    };

	const handlePasswordChange = async (username, password) => {
        setError('');
        setSuccessMessage('');
        try {
            await subsonicApiRequest('updateUser.view', { username, password });
            setEditingUser(null);
            setSuccessMessage('Password updated successfully.');
        } catch (err) {
            setError(err.message || 'Failed to update password.');
        }
    };

	const handleDelete = async (username) => {
        setError('');
        setSuccessMessage('');
		if (window.confirm(`Are you sure you want to delete user: ${username}?`)) {
			try {
                await subsonicApiRequest('deleteUser.view', { username });
				setSuccessMessage('User deleted successfully.');
				fetchUsers();
			} catch (err) {
                setError(err.message || 'Failed to delete user.');
			}
		}
	};

	return (
		<div className="bg-gray-800 p-4 sm:p-6 rounded-lg">
			<div className="flex flex-col sm:flex-row justify-between items-start sm:items-center mb-4 space-y-2 sm:space-y-0">
				<h3 className="text-xl font-bold">User Management</h3>
				<button onClick={() => setIsCreatingUser(true)} className="bg-indigo-600 hover:bg-indigo-700 text-white font-bold py-2 px-4 rounded">Create User</button>
			</div>
			{error && <p className="text-red-500 mb-4 p-3 bg-red-900/50 rounded">{error}</p>}
            {successMessage && <p className="text-green-400 mb-4 p-3 bg-green-900/50 rounded">{successMessage}</p>}
			<div className="overflow-x-auto">
				<table className="min-w-full text-sm text-left text-gray-400">
					<thead className="text-xs text-gray-300 uppercase bg-gray-700">
						<tr>
							<th scope="col" className="px-6 py-3">Username</th>
							<th scope="col" className="px-6 py-3">Admin</th>
							<th scope="col" className="px-6 py-3 text-right">Actions</th>
						</tr>
					</thead>
					<tbody>
						{users.map(user => (
							<tr key={user.username} className="bg-gray-800 border-b border-gray-700 hover:bg-gray-600">
								<td className="px-6 py-4 font-medium text-white">{user.username}</td>
								<td className="px-6 py-4">{user.is_admin ? 'Yes' : 'No'}</td>
								<td className="px-6 py-4 text-right space-x-2 whitespace-nowrap">
									<button onClick={() => setEditingUser(user)} className="font-medium text-blue-500 hover:underline">Edit Password</button>
									<button onClick={() => handleDelete(user.username)} className="font-medium text-red-500 hover:underline">Delete</button>
								</td>
							</tr>
						))}
					</tbody>
				</table>
			</div>
            {isCreatingUser && (
                <UserFormModal
                    onClose={() => setIsCreatingUser(false)}
                    onSubmit={handleCreate}
                    title="Create New User"
                />
            )}
			{editingUser && (
				<PasswordEditModal
					user={editingUser}
					onClose={() => setEditingUser(null)}
					onSubmit={handlePasswordChange}
				/>
			)}
		</div>
	);
}

const UserFormModal = ({ onClose, onSubmit, title }) => {
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [isAdmin, setIsAdmin] = useState(false);

    const handleSubmit = (e) => {
        e.preventDefault();
        onSubmit({ username, password, is_admin: isAdmin });
    };

    return (
        <Modal onClose={onClose}>
            <h3 className="text-xl font-bold mb-4">{title}</h3>
            <form onSubmit={handleSubmit}>
                <div className="mb-4">
                    <label className="block text-gray-400 mb-2">Username</label>
                    <input type="text" value={username} onChange={e => setUsername(e.target.value)} className="w-full p-2 bg-gray-700 rounded" required/>
                </div>
                <div className="mb-4">
                    <label className="block text-gray-400 mb-2">Password</label>
                    <input type="password" value={password} onChange={e => setPassword(e.target.value)} className="w-full p-2 bg-gray-700 rounded" required />
                </div>
                <div className="mb-4 flex items-center">
                    <input type="checkbox" checked={isAdmin} onChange={e => setIsAdmin(e.target.checked)} id="isAdminCheck" className="w-4 h-4 text-teal-600 bg-gray-700 border-gray-600 rounded focus:ring-teal-500" />
                    <label htmlFor="isAdminCheck" className="ml-2 text-sm font-medium text-gray-300">Is Admin?</label>
                </div>
                <div className="flex flex-col sm:flex-row justify-end space-y-2 sm:space-y-0 sm:space-x-4">
                    <button type="button" onClick={onClose} className="bg-gray-600 hover:bg-gray-700 text-white font-bold py-2 px-4 rounded">Cancel</button>
                    <button type="submit" className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Save</button>
                </div>
            </form>
        </Modal>
    );
};

const PasswordEditModal = ({ user, onClose, onSubmit }) => {
	const [password, setPassword] = useState('');
	const handleSubmit = (e) => {
		e.preventDefault();
		onSubmit(user.username, password);
	};
	return (
		<Modal onClose={onClose}>
			<h3 className="text-xl font-bold mb-4">Edit Password for {user.username}</h3>
			<form onSubmit={handleSubmit}>
				<div className="mb-4">
					<label className="block text-gray-400 mb-2">New Password</label>
					<input
						type="password"
						value={password}
						onChange={(e) => setPassword(e.target.value)}
						className="w-full p-2 bg-gray-700 rounded"
						required
					/>
				</div>
                <div className="flex flex-col sm:flex-row justify-end space-y-2 sm:space-y-0 sm:space-x-4">
                    <button type="button" onClick={onClose} className="bg-gray-600 hover:bg-gray-700 text-white font-bold py-2 px-4 rounded">Cancel</button>
                    <button type="submit" className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded">Update Password</button>
                </div>
			</form>
		</Modal>
	);
};


function SonicAnalysisPanel() {
    const [status, setStatus] = useState(null);
    const [error, setError] = useState('');
    const [isLoading, setIsLoading] = useState(true);

    const fetchStatus = useCallback(async () => {
        const token = localStorage.getItem('token');
        try {
            const response = await fetch('/api/v1/admin/analysis/status', {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (!response.ok) {
                const errData = await response.json().catch(() => ({ error: `Server error: ${response.status}` }));
                throw new Error(errData.error || `Server error: ${response.status}`);
            }
            const data = await response.json();
            setStatus(data);
            setError('');
        } catch (err) {
            setError(err.message);
            console.error("Failed to fetch analysis status:", err);
        } finally {
            setIsLoading(false);
        }
    }, []);

    useEffect(() => {
        fetchStatus();
        const intervalId = setInterval(fetchStatus, 5000); // Poll every 5 seconds
        return () => clearInterval(intervalId);
    }, [fetchStatus]);

    const handleStart = async () => {
        setError('');
        setIsLoading(true);
        const token = localStorage.getItem('token');
        try {
            const response = await fetch('/api/v1/admin/analysis/start', {
                method: 'POST',
                headers: { 
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json' 
                },
                body: JSON.stringify({}) // Send empty JSON object
            });
            if (!response.ok) {
                const errData = await response.json().catch(() => ({ error: 'Failed to start analysis' }));
                throw new Error(errData.error || 'Failed to start analysis');
            }
            await fetchStatus();
        } catch (err) {
            setError(err.message);
        } finally {
            setIsLoading(false);
        }
    };

    const handleCancel = async () => {
        if (!status || !status.task_id) {
            setError("No active task to cancel.");
            return;
        }
        setError('');
        setIsLoading(true);
        const token = localStorage.getItem('token');
        try {
            const response = await fetch(`/api/v1/admin/analysis/cancel/${status.task_id}`, {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (!response.ok) {
                 const errData = await response.json().catch(() => ({ error: 'Failed to cancel analysis' }));
                throw new Error(errData.error || 'Failed to cancel analysis');
            }
            await fetchStatus();
        } catch (err) {
            setError(err.message);
        } finally {
            setIsLoading(false);
        }
    };

    const isTaskRunning = status && (status.status === 'PROGRESS' || status.status === 'STARTED' || status.status === 'PENDING');

    return (
        <div className="bg-gray-800 p-4 sm:p-6 rounded-lg">
            <h3 className="text-xl font-bold mb-4">AudioMuse-AI Sonic Analysis</h3>
            {error && <p className="text-red-500 mb-4 p-3 bg-red-900/50 rounded">{error}</p>}
            
            <div className="flex justify-end mb-4">
                <button
                    onClick={handleStart}
                    disabled={isTaskRunning || isLoading}
                    className="bg-green-600 hover:bg-green-700 text-white font-bold py-2 px-4 rounded disabled:bg-gray-500 disabled:cursor-not-allowed"
                >
                    {isLoading && !isTaskRunning ? 'Loading...' : 'Start New Analysis'}
                </button>
                {isTaskRunning && (
                     <button
                        onClick={handleCancel}
                        disabled={isLoading}
                        className="ml-4 bg-red-600 hover:bg-red-700 text-white font-bold py-2 px-4 rounded disabled:bg-gray-500 disabled:cursor-not-allowed"
                    >
                        Cancel Analysis
                    </button>
                )}
            </div>

            {isLoading && !status && <p>Loading status...</p>}
            
            {status && (
                <div className="space-y-3">
                    <div>
                        <span className="font-semibold text-gray-400">Status: </span>
                        <span className={`font-mono px-2 py-1 rounded text-sm ${isTaskRunning ? 'bg-blue-900 text-blue-300' : (status.status === 'SUCCESS' ? 'bg-green-900 text-green-300' : 'bg-gray-700 text-gray-300')}`}>{status.status || 'N/A'}</span>
                    </div>
                    {isTaskRunning && (
                        <div>
                             <div className="w-full bg-gray-700 rounded-full h-2.5">
                                <div className="bg-teal-500 h-2.5 rounded-full" style={{ width: `${status.progress || 0}%` }}></div>
                            </div>
                            <p className="text-center text-sm mt-1">{Math.round(status.progress || 0)}%</p>
                        </div>
                    )}
                    <div>
                        <span className="font-semibold text-gray-400">Message: </span>
                        <span>{status.details?.status_message || 'No status message.'}</span>
                    </div>
                     <div>
                         <span className="font-semibold text-gray-400">Running Time: </span>
                        <span>{Math.round(status.running_time_seconds || 0)} seconds</span>
                    </div>
                    <div className="pt-2">
                        <h4 className="font-semibold text-gray-400 mb-1">Logs:</h4>
                        <div className="bg-gray-900 p-3 rounded-md h-40 overflow-y-auto font-mono text-xs text-gray-300">
                           {(status.details?.log && status.details.log.length > 0) ? status.details.log.map((line, index) => (
                               <p key={index}>{line}</p>
                           )) : <p>No logs yet.</p>}
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}

export default function AdminPanel({ onConfigChange }) {
	return (
		<div className="grid grid-cols-1 xl:grid-cols-2 gap-8">
            <div className="space-y-8">
                 <LibraryManagement onConfigChange={onConfigChange} />
                 <SonicAnalysisPanel />
                 <AutoScanManagement onConfigChange={onConfigChange} />
                 <AIConfigManagement onConfigChange={onConfigChange} />
            </div>
            <UserManagement />
        </div>
	);
}

