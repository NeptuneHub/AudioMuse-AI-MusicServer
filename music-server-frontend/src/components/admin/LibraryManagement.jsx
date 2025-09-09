import React, { useState, useEffect, useCallback, useRef } from 'react';
import Modal from '../Modal';
import FileBrowser from './FileBrowser';

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

export default LibraryManagement;