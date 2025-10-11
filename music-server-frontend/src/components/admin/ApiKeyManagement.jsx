import React, { useState, useEffect, useCallback } from 'react';
import { subsonicFetch } from '../../api';

function ApiKeyManagement() {
    const [apiKey, setApiKey] = useState('');
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState('');
    const [copySuccess, setCopySuccess] = useState('');

    const subsonicApiRequest = useCallback(async (endpoint, options = {}) => {
        if (options.method === 'POST') {
            // For POST requests, we need to use apiFetch instead
            const { apiFetch } = await import('../../api');
            const response = await apiFetch(`/rest/${endpoint}?f=json`, { method: 'POST' });
            if (!response.ok) {
                const data = await response.json().catch(() => ({}));
                const subsonicResponse = data["subsonic-response"];
                const error = subsonicResponse?.error;
                throw new Error(error?.message || `Server error: ${response.status}`);
            }
            const data = await response.json();
            return data["subsonic-response"];
        } else {
            // For GET requests, use subsonicFetch
            return await subsonicFetch(endpoint, { f: 'json' });
        }
    }, []);

    const fetchApiKey = useCallback(async () => {
        setIsLoading(true);
        setError('');
        try {
            const data = await subsonicApiRequest('getApiKey.view');
            setApiKey(data?.apiKey?.key || '');
        } catch (err) {
            setError(err.message || 'Failed to fetch API key.');
            setApiKey('');
        } finally {
            setIsLoading(false);
        }
    }, [subsonicApiRequest]);

    useEffect(() => {
        fetchApiKey();
    }, [fetchApiKey]);

    const handleRevoke = async () => {
        if (!window.confirm("Are you sure you want to revoke this API key? Any applications using it will stop working.")) return;
        
        setError('');
        setCopySuccess('');
        try {
            await subsonicApiRequest('revokeApiKey.view', { method: 'POST' });
            setApiKey(''); // Clear the key from the UI
        } catch (err) {
            setError(err.message || 'Failed to revoke API key.');
        }
    };

    const handleCopy = () => {
        if (!apiKey) return;
        // document.execCommand('copy') is used for compatibility within iframes
        const textArea = document.createElement("textarea");
        textArea.value = apiKey;
        textArea.style.position = "fixed";  //avoid scrolling to bottom
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();
        try {
            document.execCommand('copy');
            setCopySuccess('Copied to clipboard!');
            setTimeout(() => setCopySuccess(''), 2000);
        } catch (err) {
            setCopySuccess('Failed to copy!');
            console.error('Could not copy text: ', err);
        }
        document.body.removeChild(textArea);
    };

    return (
        <div className="bg-gray-800 p-4 sm:p-6 rounded-lg">
            <h3 className="text-xl font-bold mb-4">API Key Management</h3>
            <p className="text-sm text-gray-400 mb-4">
                This API key can be used by Subsonic-compatible clients for authentication. The key is tied to your user account.
            </p>
            {error && <p className="text-red-500 mb-4 p-3 bg-red-900/50 rounded">{error}</p>}
            
            <div className="space-y-4">
                <label htmlFor="api-key-input" className="block text-sm font-medium text-gray-300">Your API Key</label>
                <div className="flex flex-col sm:flex-row space-y-2 sm:space-y-0 sm:space-x-2">
                    <input
                        id="api-key-input"
                        type="text"
                        value={isLoading ? 'Loading...' : (apiKey || 'No key found. One will be generated.')}
                        readOnly
                        className="flex-grow p-2 bg-gray-900 rounded border border-gray-600 font-mono text-gray-400"
                    />
                    <button 
                        onClick={handleCopy} 
                        disabled={!apiKey || isLoading}
                        className="px-4 py-2 bg-gray-600 hover:bg-gray-700 text-white font-bold rounded disabled:opacity-50"
                    >
                        {copySuccess || 'Copy'}
                    </button>
                </div>
            </div>

             <div className="mt-4 flex flex-col sm:flex-row justify-end space-y-2 sm:space-y-0 sm:space-x-4">
                <button 
                    onClick={handleRevoke} 
                    disabled={!apiKey || isLoading}
                    className="bg-red-600 hover:bg-red-700 text-white font-bold py-2 px-4 rounded disabled:bg-red-400 disabled:cursor-not-allowed"
                >
                    Revoke Key
                </button>
                <button 
                    onClick={fetchApiKey} 
                    disabled={isLoading}
                    className="bg-teal-500 hover:bg-teal-600 text-white font-bold py-2 px-4 rounded disabled:bg-teal-400 disabled:cursor-not-allowed"
                >
                    {apiKey ? 'Refresh' : 'Generate Key'}
                </button>
            </div>
        </div>
    );
}

export default ApiKeyManagement;

